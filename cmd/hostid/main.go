// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/hostid: print numeric host identifier.
// Implements srd048-hostid R1.1, R1.2, R2.1, R2.2.
package main

/*
#include <unistd.h>
*/
import "C"

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "hostid"

// versionText is printed when --version is passed.
// R2.2: print version information to stdout and exit 0.
const versionText = progName + " (go-unix-utils)"

// helpText is the usage message printed when --help is passed.
// R2.1: print usage to stdout and exit 0.
const helpText = `Usage: hostid [OPTION]...
Print the numeric identifier (as a hexadecimal number) for the current host.

      --help        display this help and exit
      --version     output version information and exit
`

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	if len(args) > 0 {
		if handled := handleInfoFlags(args); handled {
			return
		}
		// R2.1, R2.2: reject extra operands and unknown flags.
		printExtraOperandError(args[0])
		os.Exit(1)
	}

	// R1.1, R1.2: print 32-bit host identifier as 8-digit zero-padded
	// lowercase hexadecimal, followed by a newline.
	id := getHostID()
	fmt.Printf("%08x\n", id)
}

// handleInfoFlags checks for --help and --version as the first argument.
// Returns true if a flag was handled (caller should return).
func handleInfoFlags(args []string) bool {
	switch args[0] {
	case "--help":
		fmt.Print(helpText)
		return true
	case "--version":
		fmt.Println(versionText)
		return true
	}
	return false
}

// printExtraOperandError prints the GNU-style error for extra operands or
// unknown flags.
func printExtraOperandError(arg string) {
	fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, arg)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
}

// getHostID returns the 32-bit host identifier via gethostid(3).
// R1.2: value is derived from the gethostid(3) syscall.
func getHostID() uint32 {
	return uint32(C.gethostid())
}
