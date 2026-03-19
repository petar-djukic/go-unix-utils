// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd020-echo R1.1–R1.4: core output behavior for echo.
// R1.1: arguments joined by spaces, followed by newline.
// R1.2: no arguments outputs only a newline.
// R1.3: -n suppresses trailing newline.
// R1.4: unrecognized flags are passed through as literal text.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout)
	os.Exit(exitCode)
}

// run parses flags and writes output, returning the exit code.
// R1.1: joins arguments with spaces, appends newline.
// R1.2: no arguments produces a bare newline.
func run(args []string, w *os.File) int {
	noNewline, remaining := parseFlags(args)
	output := strings.Join(remaining, " ")
	if noNewline {
		_, err := fmt.Fprint(w, output)
		if err != nil {
			return 1
		}
		return 0
	}
	_, err := fmt.Fprintln(w, output)
	if err != nil {
		return 1
	}
	return 0
}

// parseFlags extracts leading GNU echo flags from the argument list.
// GNU echo recognizes flags that consist entirely of the characters n, e, E
// (e.g., -n, -nee, -En). Any argument that does not match this pattern ends
// flag parsing and is treated as a positional argument.
// R1.3: -n suppresses the trailing newline.
// R1.4: unrecognized flags are passed through as literal text.
func parseFlags(args []string) (noNewline bool, remaining []string) {
	i := 0
	for i < len(args) {
		if !isEchoFlag(args[i]) {
			break
		}
		for _, ch := range args[i][1:] {
			if ch == 'n' {
				noNewline = true
			}
			// -e and -E are recognized but escape handling is in R2 (not this task)
		}
		i++
	}
	return noNewline, args[i:]
}

// isEchoFlag returns true if arg is a valid GNU echo flag string.
// Valid flags are "-" followed by one or more characters from {n, e, E}.
func isEchoFlag(arg string) bool {
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	for _, ch := range arg[1:] {
		if ch != 'n' && ch != 'e' && ch != 'E' {
			return false
		}
	}
	return true
}
