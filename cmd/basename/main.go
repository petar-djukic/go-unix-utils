// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd015-basename R1.1–R1.5, R2.1–R2.3, R3.1–R3.4:
// strip directory and suffix from filenames with multi-argument
// and NUL-delimited output modes.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set via ldflags at build time.
var version = "unknown"

// options holds parsed command-line flags.
type options struct {
	multiple bool
	suffix   string
	zero     bool
	version  bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, names := parseArgs(os.Args[1:])
	if opts.version {
		fmt.Printf("basename (go-unix-utils) %s\n", version)
		return
	}
	if len(names) == 0 {
		printError("missing operand")
		os.Exit(1)
	}
	runBasename(opts, names)
}

// runBasename processes names according to the parsed options and
// writes results to stdout. Implements R2.1–R2.3, R3.1.
func runBasename(opts options, names []string) {
	terminator := "\n"
	if opts.zero {
		terminator = "\x00"
	}
	if opts.multiple {
		for _, name := range names {
			fmt.Print(basename(name, opts.suffix) + terminator)
		}
		return
	}
	if len(names) > 2 {
		printError(fmt.Sprintf("extra operand '%s'", names[2]))
		os.Exit(1)
	}
	suffix := ""
	if len(names) == 2 {
		suffix = names[1]
	}
	fmt.Print(basename(names[0], suffix) + terminator)
}

// printError writes a formatted error message to stderr. Implements R3.4.
func printError(msg string) {
	fmt.Fprintf(os.Stderr,
		"basename: %s\nTry 'basename --help' for more information.\n", msg)
}

// parseArgs splits raw arguments into options and positional names.
func parseArgs(args []string) (options, []string) {
	var opts options
	var names []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			names = append(names, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			i = parseLongFlag(&opts, args, i)
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			i = parseShortFlags(&opts, args, i)
			continue
		}
		names = append(names, arg)
		i++
	}
	return opts, names
}

// parseLongFlag handles a single long-form flag (--multiple, --suffix, etc.).
func parseLongFlag(opts *options, args []string, i int) int {
	arg := args[i]
	switch {
	case arg == "--version":
		opts.version = true
	case arg == "--multiple":
		opts.multiple = true
	case arg == "--zero":
		opts.zero = true
	case strings.HasPrefix(arg, "--suffix="):
		opts.suffix = arg[len("--suffix="):]
		opts.multiple = true
	case arg == "--suffix" && i+1 < len(args):
		opts.suffix = args[i+1]
		opts.multiple = true
		return i + 2
	}
	return i + 1
}

// parseShortFlags handles combined short flags (e.g., -az, -s SUFFIX).
func parseShortFlags(opts *options, args []string, i int) int {
	arg := args[i]
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'a':
			opts.multiple = true
		case 'z':
			opts.zero = true
		case 's':
			opts.multiple = true
			if j+1 < len(arg) {
				opts.suffix = arg[j+1:]
			} else if i+1 < len(args) {
				opts.suffix = args[i+1]
				i++
			}
			return i + 1
		}
	}
	return i + 1
}

// basename strips the directory prefix and optional suffix from name,
// matching GNU coreutils behavior. Implements R1.1–R1.5.
func basename(name, suffix string) string {
	if name == "" {
		return ""
	}
	if allSlashes(name) {
		return "/"
	}
	name = strings.TrimRight(name, "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if suffix != "" && name != suffix && strings.HasSuffix(name, suffix) {
		name = name[:len(name)-len(suffix)]
	}
	return name
}

// allSlashes reports whether s is non-empty and consists entirely of '/'.
func allSlashes(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '/' {
			return false
		}
	}
	return true
}
