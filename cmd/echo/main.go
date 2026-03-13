// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/echo implements the echo command.
// Implements: prd020-echo R1.1, R1.2, R1.3, R1.4
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R3.3: Handle SIGPIPE gracefully per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3, R1.4: Parse leading flag arguments. GNU echo recognizes -n, -e, -E
	// and combinations like -neE. Once a non-flag argument is encountered,
	// all remaining arguments (including later dash-prefixed ones) are positional.
	suppressNewline := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			break
		}
		valid := true
		for _, c := range arg[1:] {
			if c != 'n' && c != 'e' && c != 'E' {
				valid = false
				break
			}
		}
		if !valid {
			break
		}
		for _, c := range arg[1:] {
			if c == 'n' {
				suppressNewline = true
			}
		}
		i++
	}

	// R1.1: Write arguments separated by spaces.
	// R1.2: No arguments produces empty output (newline appended below).
	output := strings.Join(args[i:], " ")
	if !suppressNewline {
		output += "\n"
	}

	// R1.1, R3.1, R3.2: Write to stdout; exit 1 on write failure.
	_, err := fmt.Fprint(os.Stdout, output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "echo: write error: %v\n", err)
		os.Exit(1)
	}
}
