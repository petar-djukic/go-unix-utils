// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd050-readlink R1.1–R1.4
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error output.
const programName = "readlink"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3: collect operands (no flags in scope for R1.1–R1.4).
	operands := args

	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	// R1.1, R1.3, R1.4: process each operand in order, track failures.
	exitCode := 0
	for _, path := range operands {
		// D2: use os.Readlink to read the symbolic link target.
		target, err := os.Readlink(path)
		if err != nil {
			// R1.2: not a symlink or does not exist — print error to stderr.
			// D3: error format matches GNU readlink.
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n", programName, path, readlinkErrMsg(err))
			// R1.4: continue processing remaining operands.
			exitCode = 1
			continue
		}
		// R1.1: print the target followed by a newline.
		fmt.Println(target)
	}
	os.Exit(exitCode)
}

// readlinkErrMsg extracts a user-facing error message from an os.Readlink error.
// os.Readlink wraps the error in *os.PathError; we unwrap to get the syscall message.
func readlinkErrMsg(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
