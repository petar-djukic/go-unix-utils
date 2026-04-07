// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/basename: strip directory and suffix from filenames.
// Implements srd015-basename R1.1-R1.5, R2.1-R2.3, R3.1-R3.4.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "basename"

func main() {
	sys.InstallSIGPIPEHandler()

	opts, names, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		// R3.4: print usage hint to stderr matching GNU basename format.
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	terminator := "\n"
	if opts.zero {
		terminator = "\x00"
	}

	suffix := opts.suffix
	// R1.2: in single-argument mode, second positional arg is the suffix.
	if !opts.multiple && len(names) == 2 {
		suffix = names[1]
		names = names[:1]
	}

	for _, name := range names {
		result := basename(name, suffix)
		fmt.Print(result + terminator)
	}
}

// options holds parsed command-line flags.
type options struct {
	multiple bool
	suffix   string
	zero     bool
}

// parseArgs parses flags and positional arguments.
// Returns options, the list of NAME arguments, and any error.
func parseArgs(args []string) (options, []string, error) {
	var opts options
	var names []string
	skipNext := false

	for i := range len(args) {
		if skipNext {
			skipNext = false
			continue
		}
		arg := args[i]
		if arg == "--" {
			names = append(names, args[i+1:]...)
			break
		}
		if parsed, skip := parseLongFlag(arg, args, i, &opts); parsed {
			skipNext = skip
			continue
		}
		if parsed, skip := parseShortFlags(arg, args, i, &opts); parsed {
			skipNext = skip
			continue
		}
		names = append(names, args[i:]...)
		break
	}

	return opts, names, validateArgs(opts, names)
}

// parseLongFlag handles --multiple, --suffix=SUFFIX, --suffix SUFFIX, --zero.
// Returns (handled, skipNext).
func parseLongFlag(
	arg string, args []string, i int, opts *options,
) (bool, bool) {
	switch {
	case arg == "--multiple":
		opts.multiple = true
		return true, false
	case arg == "--zero":
		opts.zero = true
		return true, false
	case arg == "--suffix":
		if i+1 < len(args) {
			opts.suffix = args[i+1]
			opts.multiple = true
			return true, true
		}
		return true, false
	case strings.HasPrefix(arg, "--suffix="):
		opts.suffix = arg[len("--suffix="):]
		opts.multiple = true
		return true, false
	default:
		return false, false
	}
}

// parseShortFlags handles -a, -s SUFFIX, -z, and combined short flags.
// Returns (handled, skipNext).
func parseShortFlags(
	arg string, args []string, i int, opts *options,
) (bool, bool) {
	if len(arg) < 2 || arg[0] != '-' {
		return false, false
	}
	skipNext := false
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'a':
			opts.multiple = true
		case 'z':
			opts.zero = true
		case 's':
			opts.multiple = true
			// Remainder of this arg or next arg is the suffix value.
			if j+1 < len(arg) {
				opts.suffix = arg[j+1:]
			} else if i+1 < len(args) {
				opts.suffix = args[i+1]
				skipNext = true
			}
			return true, skipNext
		default:
			return false, false
		}
	}
	return true, skipNext
}

// validateArgs checks positional argument count against the mode.
func validateArgs(opts options, names []string) error {
	if len(names) == 0 {
		return fmt.Errorf("%s: missing operand", progName)
	}
	// R2.1/R2.2: multi-argument mode accepts any count >= 1.
	if opts.multiple {
		return nil
	}
	// Single-argument mode: 1 NAME or 1 NAME + 1 SUFFIX.
	if len(names) > 2 {
		return fmt.Errorf("%s: extra operand '%s'", progName, names[2])
	}
	return nil
}

// basename strips the directory prefix and optional suffix from name.
// R1.3: trailing slashes are removed before processing.
// R1.4: a name consisting entirely of slashes returns "/".
// R1.1: the longest prefix ending in '/' is stripped.
// R1.2: suffix is removed from the end if it matches (literal).
// R1.5: empty string produces empty output.
func basename(name, suffix string) string {
	if name == "" {
		return ""
	}

	// R1.3: strip trailing slashes.
	trimmed := strings.TrimRight(name, "/")

	// R1.4: if name was entirely slashes, result is "/".
	if trimmed == "" {
		return "/"
	}
	name = trimmed

	// R1.1: strip longest prefix ending in '/'.
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}

	// R1.2: remove suffix if present and not equal to the entire name.
	if suffix != "" && name != suffix && strings.HasSuffix(name, suffix) {
		name = name[:len(name)-len(suffix)]
	}

	return name
}
