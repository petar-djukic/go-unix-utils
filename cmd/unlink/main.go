// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/unlink implements GNU unlink: remove a single file via unlink(2).
//
// Implements prd038-unlink R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3.
package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "unlink"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run executes the unlink logic and returns the exit code.
// R1.1: calls syscall.Unlink on exactly one FILE argument.
// R1.2: produces no stdout output on success.
// R1.3: exits 0 on success.
// R2.1: exits non-zero with error on zero arguments.
// R2.2: exits non-zero with error on more than one argument.
// R2.3: exits non-zero with error when FILE does not exist.
// R2.4: exits non-zero with error when FILE is a directory.
func run(args []string, stderr *os.File) int {
	if len(args) == 0 {
		// R2.1: missing operand error.
		fmt.Fprintf(stderr, "%s: missing operand\n", progName)                   //nolint:errcheck
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
		return 1
	}

	if len(args) > 1 {
		// R2.2: extra operand error.
		fmt.Fprintf(stderr, "%s: extra operand '%s'\n", progName, args[1])       //nolint:errcheck
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
		return 1
	}

	// R1.1: call unlink(2) on the single FILE argument.
	// Use syscall.Unlink directly to match GNU behavior: unlink(2) refuses
	// directories on standard systems (R2.4), and returns ENOENT for
	// nonexistent files (R2.3), unlike os.Remove which falls back to rmdir.
	if err := syscall.Unlink(args[0]); err != nil {
		fmt.Fprintf(stderr, "%s: cannot unlink '%s': %s\n", progName, args[0], err.Error()) //nolint:errcheck
		return 1
	}

	// R1.2, R1.3: no output, exit 0.
	return 0
}
