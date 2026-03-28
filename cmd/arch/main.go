// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd045-arch: Print Machine Hardware Name.
// Covers R1.1-R1.2 (default behavior, uname -m parity),
// R2.1-R2.2 (extra operand/unknown flag errors),
// R3.1-R3.2 (differential testing).
package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

// run parses arguments and prints the machine hardware name. Returns exit code.
func run(args []string) int {
	for _, arg := range args {
		switch arg {
		case "--help":
			return printHelp()
		case "--version":
			return printVersion()
		case "--":
			// Ignore, but any following args are extra operands.
		default:
			if len(arg) > 1 && arg[0] == '-' {
				// R2.2: unknown flags produce an error.
				fmt.Fprintf(os.Stderr, "arch: unrecognized option '%s'\n", arg)
				fmt.Fprintln(os.Stderr, "Try 'arch --help' for more information.")
				return 1
			}
			// R2.1: extra operand.
			fmt.Fprintf(os.Stderr, "arch: extra operand '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'arch --help' for more information.")
			return 1
		}
	}

	// R1.1, R1.2: print machine hardware name (same as uname -m).
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		fmt.Fprintf(os.Stderr, "arch: %v\n", err)
		return 1
	}

	machine := utsToString(utsname.Machine)
	if _, err := fmt.Println(machine); err != nil {
		return 1
	}
	return 0
}

// utsToString converts a Utsname byte array field to a Go string.
func utsToString(field [256]byte) string {
	n := 0
	for n < len(field) && field[n] != 0 {
		n++
	}
	return string(field[:n])
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: arch [OPTION]...
Print machine hardware name.

      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "arch (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
