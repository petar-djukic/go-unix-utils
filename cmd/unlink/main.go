// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/unlink: remove a single file via unlink(2).
// Implements srd038 R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4.
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

	// R2.4: refuse to unlink directories with GNU-style error message.
	if isDirectory(file) {
		fmt.Fprintf(os.Stderr, "%s: cannot unlink '%s': Is a directory\n",
			programName, file)
		os.Exit(1)
	}

	// R1.1: call unlink(2) on the single argument.
	// R2.3: permission errors and non-existent files reported via OS error.
	// R2.4 (symlinks): syscall.Unlink removes the symlink itself, not its target.
	if err := syscall.Unlink(file); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot unlink '%s': %s\n",
			programName, file, err.Error())
		os.Exit(1)
	}
	// R1.2: exit 0 on success, no stdout output.
}

// isDirectory returns true if path is a directory (not following symlinks).
func isDirectory(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
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
