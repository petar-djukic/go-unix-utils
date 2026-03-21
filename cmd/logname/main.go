// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd053-logname R1.1 (print login name with trailing newline),
// R1.2 (login name from system login record),
// R2.1 (extra operands produce error exit 1),
// R2.2 (unknown flags produce error exit 1),
// R2.3 (login name unavailable produces error exit 1).
package main

/*
#include <unistd.h>
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is used in error messages.
const programName = "logname"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run processes arguments and prints the login name.
// Returns the exit code.
func run(args []string) int {
	if err := parseArgs(args); err != nil {
		printError(err.Error())
		return 1
	}
	name, err := getLoginName()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		return 1
	}
	fmt.Println(name)
	return 0
}

// getLoginName retrieves the login name from the system login record
// via getlogin(3). R1.2: uses the system login record, not the effective UID.
func getLoginName() (string, error) {
	p := C.getlogin()
	if p == nil {
		return "", fmt.Errorf("no login name")
	}
	return C.GoString(p), nil
}

// parseArgs validates command-line arguments.
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
	if strings.HasPrefix(first, "--") {
		return fmt.Errorf("unrecognized option '%s'", first)
	}
	if strings.HasPrefix(first, "-") {
		return fmt.Errorf("invalid option -- '%c'", first[1])
	}
	return fmt.Errorf("extra operand '%s'", first)
}

// helpTextFor returns text for --help or --version.
func helpTextFor(flag string) string {
	if flag == "--version" {
		return "logname (go-unix-utils) 1.0\n"
	}
	return `Usage: logname [OPTION]
Print the name of the current user.

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
