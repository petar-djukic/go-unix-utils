// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/echo: display a line of text.
// Implements srd020-echo R1.1, R1.2, R1.3, R1.4.
package main

import (
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R3.3: install SIGPIPE handler so echo exits 0 when pipe closes.
	sys.InstallSIGPIPEHandler()

	noNewline, args := parseFlags(os.Args[1:])

	output := buildOutput(args, noNewline)

	if _, err := os.Stdout.WriteString(output); err != nil {
		os.Exit(1)
	}
}

// parseFlags performs GNU echo-style flag parsing.
// R1.3: -n suppresses trailing newline.
// R1.4: only -n, -e, -E (and combinations) are recognized as flags.
// Unrecognized flags are treated as positional arguments.
// Flag parsing stops at the first non-flag argument.
func parseFlags(args []string) (noNewline bool, rest []string) {
	i := 0
	for i < len(args) {
		if !isValidFlagArg(args[i]) {
			break
		}
		if strings.ContainsRune(args[i][1:], 'n') {
			noNewline = true
		}
		i++
	}
	return noNewline, args[i:]
}

// isValidFlagArg checks whether an argument is a valid GNU echo flag group.
// A valid flag argument starts with '-' followed by one or more of [neE].
func isValidFlagArg(arg string) bool {
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	for _, c := range arg[1:] {
		if c != 'n' && c != 'e' && c != 'E' {
			return false
		}
	}
	return true
}

// buildOutput constructs the output string from positional arguments.
// R1.1: arguments joined by spaces, followed by newline.
// R1.2: no arguments produces only a newline.
// R1.3: noNewline suppresses the trailing newline.
func buildOutput(args []string, noNewline bool) string {
	result := strings.Join(args, " ")
	if !noNewline {
		result += "\n"
	}
	return result
}
