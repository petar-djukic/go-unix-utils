// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd015-basename: Strip Directory and Suffix from Filenames.
// Covers R1.1-R1.5 (core path extraction, suffix removal, trailing slashes),
// R2.1-R2.3 (multi-argument mode, -s/--suffix flag),
// R3.1-R3.4 (NUL termination, exit codes, error messages).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, names, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	// R3.3, R3.4: no arguments is an error.
	if len(names) == 0 {
		fmt.Fprintln(os.Stderr, "basename: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'basename --help' for more information.")
		os.Exit(1)
	}

	exitCode = run(cfg, names)
	os.Exit(exitCode)
}

// config holds parsed flag state.
type config struct {
	suffix string
	multi  bool
	zero   bool
}

// run processes the names and prints results. Returns exit code.
func run(cfg config, names []string) int {
	// R1.2: two-argument form treats second positional as suffix.
	if !cfg.multi && len(names) == 2 {
		cfg.suffix = names[1]
		names = names[:1]
	} else if !cfg.multi && len(names) > 2 {
		// R3.3, R3.4: extra operand in single-argument mode.
		fmt.Fprintf(os.Stderr, "basename: extra operand '%s'\n", names[2])
		fmt.Fprintln(os.Stderr, "Try 'basename --help' for more information.")
		return 1
	}

	terminator := "\n"
	if cfg.zero {
		// R3.1: NUL byte instead of newline.
		terminator = "\x00"
	}

	for _, name := range names {
		if _, err := fmt.Print(basename(name, cfg.suffix) + terminator); err != nil {
			return 1
		}
	}
	return 0
}

// basename strips the directory component and optional suffix from name.
// R1.1: strips longest prefix ending in '/'.
// R1.3: trailing slashes are removed before processing.
func basename(name, suffix string) string {
	// R1.5: empty string produces empty result.
	if name == "" {
		return ""
	}

	// R1.3: strip trailing slashes.
	trimmed := strings.TrimRight(name, "/")

	// R1.4: name was entirely slashes.
	if trimmed == "" {
		return "/"
	}

	// R1.1: strip directory component.
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		trimmed = trimmed[i+1:]
	}

	// R1.2: remove suffix if present and result is not equal to suffix.
	if suffix != "" && trimmed != suffix && strings.HasSuffix(trimmed, suffix) {
		trimmed = trimmed[:len(trimmed)-len(suffix)]
	}

	return trimmed
}

// parseArgs processes flags and returns configuration.
// exit is -1 when processing should continue; >= 0 for early termination.
func parseArgs(args []string) (cfg config, names []string, exit int) {
	exit = -1
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			names = append(names, args[i+1:]...)
			return
		case arg == "--help":
			return config{}, nil, printHelp()
		case arg == "--version":
			return config{}, nil, printVersion()
		case arg == "-a" || arg == "--multiple":
			// R2.1: enable multi-argument mode.
			cfg.multi = true
		case arg == "-z" || arg == "--zero":
			// R3.1: NUL-terminated output.
			cfg.zero = true
		case arg == "-s":
			// R2.2: -s SUFFIX implies -a.
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "basename: option requires an argument -- 's'")
				return config{}, nil, 1
			}
			cfg.suffix = args[i]
			cfg.multi = true
		case strings.HasPrefix(arg, "--suffix="):
			// R2.2: --suffix=SUFFIX implies -a.
			cfg.suffix = strings.TrimPrefix(arg, "--suffix=")
			cfg.multi = true
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			fmt.Fprintf(os.Stderr, "basename: unrecognized option '%s'\n", arg)
			return config{}, nil, 1
		default:
			// First non-flag argument; remaining args are all names.
			names = append(names, args[i:]...)
			return
		}
	}
	return
}

// printHelp writes usage information to stdout and returns the exit code.
// R2.1: --help prints to stdout and exits 0.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: basename NAME [SUFFIX]
  or:  basename OPTION... NAME...
Print NAME with any leading directory components removed.
If specified, also remove a trailing SUFFIX.

Mandatory arguments to long options are mandatory for short options too.
  -a, --multiple       support multiple arguments and treat each as a NAME
  -s, --suffix=SUFFIX  remove a trailing SUFFIX; implies -a
  -z, --zero           end each output line with NUL, not newline

      --help     display this help and exit
      --version  output version information and exit

Examples:
  basename /usr/bin/sort          -> "sort"
  basename include/stdio.h .h     -> "stdio"
  basename -s .h include/stdio.h  -> "stdio"
  basename -a any/str1 any/str2   -> "str1" followed by "str2"
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
// R2.2: --version prints to stdout and exits 0.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "basename (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
