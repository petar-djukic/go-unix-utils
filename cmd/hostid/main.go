// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd048-hostid R1.1 (print host identifier as 8-digit hex),
// R1.2 (value derived from gethostid(3)),
// R2.1 (extra operands produce error exit 1),
// R2.2 (unknown flags produce error exit 1),
// R3.1–R3.3 (differential testing against ghostid).
package main

/*
#include <unistd.h>
*/
import "C"

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is used in error messages.
const programName = "hostid"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run processes arguments and prints the host identifier.
// Returns the exit code.
func run(args []string) int {
	if err := parseArgs(args); err != nil {
		printError(err.Error())
		return 1
	}
	// R1.1, R1.2: print 32-bit host identifier as 8-digit lowercase hex.
	id := C.gethostid()
	fmt.Printf("%08x\n", uint32(id))
	return 0
}

// parseArgs validates command-line arguments.
// Only --help and --version are accepted; all other args are errors.
// R2.1: extra operands produce error. R2.2: unknown flags produce error.
func parseArgs(args []string) error {
	if len(args) == 0 {
		return nil
	}
	first := args[0]
	if first == "--" {
		if len(args) > 1 {
			return fmt.Errorf("extra operand '%s'", args[1])
		}
		return nil
	}
	if first == "--help" || first == "--version" {
		fmt.Print(helpTextFor(first))
		os.Exit(0)
	}
	if strings.HasPrefix(first, "-") {
		return fmt.Errorf("unrecognized option '%s'", first)
	}
	return fmt.Errorf("extra operand '%s'", first)
}

// helpTextFor returns text for --help or --version.
func helpTextFor(flag string) string {
	if flag == "--version" {
		return "hostid (go-unix-utils) 1.0\n"
	}
	return `Usage: hostid [OPTION]
Print the numeric host identifier.

      --help     display this help and exit
      --version  output version information and exit
`
}

// printError writes a formatted error message to stderr.
func printError(msg string) {
	fmt.Fprintf(os.Stderr,
		"%s: %s\nTry '%s --help' for more information.\n",
		programName, msg, programName)
}
