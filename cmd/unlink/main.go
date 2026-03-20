// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd038-unlink R1.1–R1.3, R2.1–R2.4: unlink a single file via
// the unlink(2) syscall, with argument validation and error reporting matching
// GNU unlink behavior.
package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the program name used in error messages.
const progName = "unlink"

// helpText is the usage message printed for --help.
const helpText = `Usage: unlink FILE
  or:  unlink OPTION
Call the unlink function to remove the specified FILE.

      --help        display this help and exit
      --version     output version information and exit
`

// versionText is the version message printed for --version.
const versionText = `unlink (go-unix-utils) 1.0
`

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3: --help and --version handling.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			printAndExit(helpText)
		case "--version":
			printAndExit(versionText)
		}
	}

	// R1.2 / R2.1: zero arguments — missing operand.
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\nTry '%s --help' for more information.\n", progName, progName)
		os.Exit(1)
	}

	// R1.2 / R2.2: more than one argument — extra operand.
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\nTry '%s --help' for more information.\n", progName, args[1], progName)
		os.Exit(1)
	}

	// R1.1: call unlink(2) directly on exactly one file argument.
	// Uses syscall.Unlink (not os.Remove) to avoid Go's fallback to rmdir
	// for empty directories — GNU unlink calls unlink(2) only.
	if err := syscall.Unlink(args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot unlink '%s': %s\n", progName, args[0], err)
		os.Exit(1)
	}
}

// printAndExit writes text to stdout and exits 0 on success or 1 on write error.
func printAndExit(text string) {
	_, err := fmt.Fprint(os.Stdout, text)
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
