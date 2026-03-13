// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd016-dirname R1.1–R1.5, R2.1, R2.2
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error output.
const programName = "dirname"

func main() {
	// D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// Parse flags: -z/--zero.
	var zero bool
	var operands []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			// End of flags; remaining args are operands.
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case arg == "-z" || arg == "--zero":
			// R2.1: NUL-terminated output.
			zero = true
		case strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-':
			// Short flag cluster: e.g. -z
			cluster := arg[1:]
			for j := 0; j < len(cluster); j++ {
				switch cluster[j] {
				case 'z':
					zero = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, cluster[j])
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
					os.Exit(1)
				}
			}
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		default:
			operands = append(operands, arg)
		}
	}

	// R3.2: no arguments is an error.
	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	// R2.1: choose line terminator.
	terminator := "\n"
	if zero {
		terminator = "\x00"
	}

	// R1.5, R2.2: process each argument in order and print one result per line.
	for _, name := range operands {
		result := dirname(name)
		fmt.Print(result + terminator)
	}
}

// dirname extracts the directory component from name, implementing the POSIX
// dirname algorithm.
//
// R1.1: Strip trailing slashes, then remove the last component.
// R1.2: If name contains no '/' after trailing-slash removal, return ".".
// R1.3: Trailing slashes are stripped before extracting the directory component.
// R1.4: Handle root path (/) and all-slash inputs by returning "/".
func dirname(name string) string {
	// R1.3: strip trailing slashes.
	stripped := strings.TrimRight(name, "/")

	// R1.4: all slashes or empty → "/". Empty input per GNU dirname also yields ".".
	if stripped == "" {
		if name == "" {
			return "."
		}
		return "/"
	}

	// R1.2: no slash means no directory component → ".".
	i := strings.LastIndex(stripped, "/")
	if i < 0 {
		return "."
	}

	// R1.1: take everything before the last '/'.
	dir := stripped[:i]

	// R1.4: strip trailing slashes from result; if empty, return "/".
	dir = strings.TrimRight(dir, "/")
	if dir == "" {
		return "/"
	}
	return dir
}
