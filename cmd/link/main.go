// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/link: create a hard link via link(2).
// Implements srd084 R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "link"

// main entry point with SIGPIPE handler and argument validation.
// R2.3: InstallSIGPIPEHandler for graceful SIGPIPE exit.
func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3: exactly two arguments required.
	if len(args) != 2 {
		printUsageError(args)
		os.Exit(1)
	}

	// R1.1, R1.2: create hard link using link(2) semantics via os.Link.
	if err := os.Link(args[0], args[1]); err != nil {
		printLinkError(err)
		os.Exit(1)
	}
	// R2.1: exit 0 on success, no stdout output.
}

// printUsageError prints the appropriate error for wrong argument count.
// R1.3: matches GNU link error format.
func printUsageError(args []string) {
	switch len(args) {
	case 0:
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
	case 1:
		fmt.Fprintf(os.Stderr, "%s: missing operand after '%s'\n",
			programName, args[0])
	default:
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n",
			programName, args[2])
	}
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n",
		programName)
}

// printLinkError prints a link(2) failure message to stderr.
// R1.4: matches GNU link error format for syscall failures.
func printLinkError(err error) {
	// os.Link returns *os.LinkError with Op, Old, New, Err fields.
	if le, ok := err.(*os.LinkError); ok {
		fmt.Fprintf(os.Stderr, "%s: cannot create link '%s' to '%s': %s\n",
			programName, le.New, le.Old, le.Err.Error())
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err.Error())
}
