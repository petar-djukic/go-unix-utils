// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/basename implements GNU basename: strip directory and suffix from filenames.
//
// Implements prd015-basename R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3, R3.1.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// helpText is the usage message printed when --help is passed.
const helpText = `Usage: basename NAME [SUFFIX]
  or:  basename OPTION... NAME...
Print NAME with any leading directory components removed.
If specified, also remove a trailing SUFFIX.

Mandatory arguments to long options are mandatory for short options too.
  -a, --multiple       support multiple arguments and treat each as a NAME
  -s, --suffix=SUFFIX  remove a trailing SUFFIX; implies -a
  -z, --zero           end each output line with NUL, not newline
      --help        display this help and exit
      --version     output version information and exit

Examples:
  basename /usr/bin/sort          -> "sort"
  basename include/stdio.h .h     -> "stdio"
  basename -s .h include/stdio.h  -> "stdio"
  basename -a any/str1 any/str2   -> "str1" followed by "str2"
`

const versionText = "basename (go-unix-utils) 0.1\n"

// parseResult signals how argument parsing concluded.
type parseResult int

const (
	parseOK   parseResult = iota
	parseHelp             // --help requested
	parseVer              // --version requested
	parseErr              // parse error
)

// options holds parsed command-line flags.
type options struct {
	multiple bool
	suffix   string
	zero     bool
	names    []string
	errMsg   string
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and executes basename logic.
// Returns exit code: 0 on success, 1 on error.
func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		printMissingOperand(stderr)
		return 1
	}

	opts, result := parseArgs(args)
	switch result {
	case parseHelp:
		fmt.Fprint(stdout, helpText) //nolint:errcheck // best-effort
		return 0
	case parseVer:
		fmt.Fprint(stdout, versionText) //nolint:errcheck // best-effort
		return 0
	case parseErr:
		fmt.Fprintln(stderr, opts.errMsg) //nolint:errcheck // best-effort
		return 1
	}

	if len(opts.names) == 0 {
		printMissingOperand(stderr)
		return 1
	}

	return printResults(opts, stdout, stderr)
}

// printMissingOperand writes the missing-operand error to stderr.
func printMissingOperand(stderr *os.File) {
	fmt.Fprintln(stderr, "basename: missing operand")              //nolint:errcheck
	fmt.Fprintln(stderr, "Try 'basename --help' for more information.") //nolint:errcheck
}

// parseArgs parses GNU-style arguments into options.
func parseArgs(args []string) (*options, parseResult) {
	opts := &options{}
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--help" {
			return opts, parseHelp
		}
		if arg == "--version" {
			return opts, parseVer
		}
		if arg == "--" {
			opts.names = append(opts.names, args[i+1:]...)
			break
		}
		if handled, adv, ok := parseLongFlag(arg, args, i, opts); handled {
			if !ok {
				return opts, parseErr
			}
			i += adv
			continue
		}
		if handled, adv, ok := parseShortFlags(arg, args, i, opts); handled {
			if !ok {
				return opts, parseErr
			}
			i += adv
			continue
		}
		// Not a flag — rest are names
		opts.names = append(opts.names, args[i:]...)
		break
	}
	return opts, parseOK
}

// parseLongFlag handles --multiple, --suffix=SUFFIX, --zero.
func parseLongFlag(arg string, args []string, i int, opts *options) (bool, int, bool) {
	switch {
	case arg == "--multiple":
		opts.multiple = true
		return true, 1, true
	case arg == "--zero":
		opts.zero = true
		return true, 1, true
	case arg == "--suffix" && i+1 < len(args):
		opts.suffix = args[i+1]
		opts.multiple = true // R2.2: -s implies -a
		return true, 2, true
	case strings.HasPrefix(arg, "--suffix="):
		opts.suffix = arg[len("--suffix="):]
		opts.multiple = true // R2.2: -s implies -a
		return true, 1, true
	case strings.HasPrefix(arg, "--"):
		opts.errMsg = fmt.Sprintf("basename: unrecognized option '%s'", arg)
		return true, 1, false
	}
	return false, 0, true
}

// parseShortFlags handles -a, -s, -z and combined short flags like -az.
func parseShortFlags(arg string, args []string, i int, opts *options) (bool, int, bool) {
	if len(arg) < 2 || arg[0] != '-' {
		return false, 0, true
	}
	adv := 1
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'a':
			opts.multiple = true
		case 'z':
			opts.zero = true
		case 's':
			opts.multiple = true // R2.2: -s implies -a
			if j+1 < len(arg) {
				opts.suffix = arg[j+1:]
				return true, adv, true
			}
			if i+1 < len(args) {
				opts.suffix = args[i+1]
				adv = 2
			}
			return true, adv, true
		default:
			opts.errMsg = fmt.Sprintf("basename: invalid option -- '%c'", arg[j])
			return true, 1, false
		}
	}
	return true, adv, true
}

// printResults outputs basename results for all names.
func printResults(opts *options, stdout, stderr *os.File) int {
	delim := "\n"
	if opts.zero {
		delim = "\x00"
	}

	if !opts.multiple {
		return printSingleMode(opts, delim, stdout, stderr)
	}

	for _, name := range opts.names {
		result := basename(name, opts.suffix)
		fmt.Fprint(stdout, result+delim) //nolint:errcheck // best-effort
	}
	return 0
}

// printSingleMode handles the non-multiple (default) invocation.
func printSingleMode(opts *options, delim string, stdout, stderr *os.File) int {
	names := opts.names
	if len(names) > 2 {
		fmt.Fprintf(stderr, "basename: extra operand '%s'\n", names[2])
		fmt.Fprintln(stderr, "Try 'basename --help' for more information.")
		return 1
	}

	name := names[0]
	suffix := opts.suffix
	if suffix == "" && len(names) == 2 {
		suffix = names[1]
	}

	result := basename(name, suffix)
	fmt.Fprint(stdout, result+delim) //nolint:errcheck // best-effort
	return 0
}

// basename strips directory components and optional suffix from name.
// R1.1: strips the longest prefix ending in '/'.
// R1.3: trailing slashes are removed first.
// R1.4: if name consists entirely of slashes, returns "/".
// R1.5: if name is empty, returns "".
func basename(name, suffix string) string {
	if name == "" {
		return ""
	}

	if allSlashes(name) {
		return "/"
	}

	name = strings.TrimRight(name, "/")

	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	if suffix != "" && suffix != name && strings.HasSuffix(name, suffix) {
		name = name[:len(name)-len(suffix)]
	}

	return name
}

// allSlashes returns true if s consists entirely of '/' characters.
func allSlashes(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '/' {
			return false
		}
	}
	return true
}
