// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/unlink: remove a single file via unlink(2).
// Implements srd038 R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3.
package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "unlink"

// main entry point with SIGPIPE handler and argument validation.
func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3, R2.1, R2.2: exactly one argument required.
	if len(args) != 1 {
		printUsageError(args)
		os.Exit(1)
	}

	file := args[0]

	// R1.1: call unlink(2) on the single argument.
	// R2.3: non-existent files reported via OS error.
	// R2.4: directories cause EPERM on macOS, matching gunlink behavior.
	if err := syscall.Unlink(file); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot unlink '%s': %s\n",
			programName, file, err.Error())
		os.Exit(1)
	}
	// R1.2: exit 0 on success, no stdout output.
}

// printUsageError prints the appropriate error for wrong argument count.
func printUsageError(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
	} else {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n",
			programName, args[1])
	}
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n",
		programName)
}
