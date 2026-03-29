// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/echo implements GNU echo: display a line of text.
// Implements prd020-echo R1.1-R1.4.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	noNewline, args := parseFlags(args)

	output := strings.Join(args, " ")
	if noNewline {
		_, err := fmt.Fprint(os.Stdout, output)
		if err != nil {
			os.Exit(1)
		}
	} else {
		_, err := fmt.Fprintln(os.Stdout, output)
		if err != nil {
			os.Exit(1)
		}
	}
}

// parseFlags extracts recognized GNU echo flags from leading arguments.
// R1.3: -n suppresses trailing newline.
// R1.4: Only -n, -e, -E (and combinations) are recognized flags.
// Unrecognized flags are treated as positional arguments.
func parseFlags(args []string) (noNewline bool, remaining []string) {
	i := 0
	for i < len(args) {
		arg := args[i]
		if !isEchoFlag(arg) {
			break
		}
		for _, ch := range arg[1:] {
			if ch == 'n' {
				noNewline = true
			}
			// -e and -E are recognized but not acted on in R1 scope.
		}
		i++
	}
	return noNewline, args[i:]
}

// isEchoFlag returns true if arg is a recognized GNU echo flag string.
// A valid flag starts with '-' followed by one or more of 'n', 'e', 'E'.
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
