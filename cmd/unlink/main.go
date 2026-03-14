// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd038-unlink R1.1–R1.3, R2.1–R2.4
package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and --help output.
const programName = "unlink"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R2.1: --help and --version are the only recognized flags.
	if len(args) == 1 {
		switch args[0] {
		case "--help":
			printHelp()
			return
		case "--version":
			printVersion()
			return
		}
	}

	// R2.1: zero arguments is an error.
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	// R2.2: more than one argument is an error.
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", programName, args[1])
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	// R1.1: call unlink(2) on exactly one FILE argument.
	// Use syscall.Unlink directly to match GNU unlink behavior — on macOS,
	// unlink(2) returns EPERM for directories, matching the GNU reference binary.
	// R2.3, R2.4: errors for non-existent files and directories are reported.
	if err := syscall.Unlink(args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot unlink '%s': %s\n", programName, args[0], err.Error())
		os.Exit(1)
	}

	// R1.2: no output on stdout when the operation succeeds.
	// R1.3: exit 0 on success.
}

// printHelp writes usage information to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: unlink FILE
  or:  unlink OPTION
Call the unlink function to remove the specified FILE.

      --help     display this help and exit
      --version  output version information and exit
`)
}

// printVersion writes version information to stdout and exits 0.
func printVersion() {
	fmt.Println("unlink (go-unix-utils) 0.1")
}
