// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the unlink utility for removing a single file.
//
// Implements prd038-unlink: basic file removal via unlink(2) (R1),
// error handling for invalid usage (R2), differential testing (R3).
package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "unlink: missing operand\nTry 'unlink --help' for more information.\n")
		os.Exit(1)
	}

	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "unlink: extra operand '%s'\nTry 'unlink --help' for more information.\n", args[1])
		os.Exit(1)
	}

	// R1.1: Call unlink(2) on exactly one FILE argument.
	// Use syscall.Unlink directly — os.Remove falls back to rmdir for
	// directories, which would diverge from GNU unlink behavior.
	if err := syscall.Unlink(args[0]); err != nil {
		reason := capitalizeFirst(err.Error())
		fmt.Fprintf(os.Stderr, "unlink: cannot unlink '%s': %s\n", args[0], reason)
		os.Exit(1)
	}
}

// capitalizeFirst returns s with the first byte uppercased.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}
