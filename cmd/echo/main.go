// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd020-echo R1.1-R1.4:
// cmd/echo prints its arguments to stdout separated by spaces, followed by
// a newline. The -n flag suppresses the trailing newline. Unrecognized flags
// are passed through as literal text. Installs SIGPIPE handler for clean
// exit on broken pipe.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R3.3 (prd): install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.4: GNU echo recognizes only -n, -e, -E and combinations thereof
	// (e.g., -nE, -neE) as flags. Any argument starting with '-' that
	// contains characters outside {n, e, E} is treated as a literal operand.
	// Flag parsing stops at the first non-flag argument.
	suppressNewline := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if len(arg) < 2 || arg[0] != '-' {
			break
		}
		// Check every character after '-' is one of n, e, E.
		validFlag := true
		for j := 1; j < len(arg); j++ {
			switch arg[j] {
			case 'n', 'e', 'E':
				// valid flag character
			default:
				validFlag = false
			}
		}
		if !validFlag {
			break
		}
		// R1.3: apply -n if present in this flag group.
		if strings.ContainsRune(arg, 'n') {
			suppressNewline = true
		}
		// -e and -E are parsed but not acted on in this task scope (R1 only).
		i++
	}

	// R1.1: print remaining arguments separated by single spaces.
	// R1.2: with no arguments, output is empty (newline handled below).
	output := strings.Join(args[i:], " ")

	// R1.1, R1.3: append newline unless -n was given.
	if !suppressNewline {
		output += "\n"
	}

	// R3.1, R3.2: exit 0 on success, >0 on write error.
	_, err := fmt.Fprint(os.Stdout, output)
	if err != nil {
		os.Exit(1)
	}
}
