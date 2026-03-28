// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd047-hostname: Print System Hostname.
// Covers R1.1-R1.2 (default behavior, gethostname parity),
// R2.1-R2.2 (extra operand/unknown flag errors),
// R3.1-R3.2 (differential testing).
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

// run parses arguments and prints the system hostname. Returns exit code.
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
				fmt.Fprintf(os.Stderr, "hostname: unrecognized option '%s'\n", arg)
				fmt.Fprintln(os.Stderr, "Try 'hostname --help' for more information.")
				return 1
			}
			// R2.1: extra operand.
			fmt.Fprintf(os.Stderr, "hostname: extra operand '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'hostname --help' for more information.")
			return 1
		}
	}

	// R1.1, R1.2: print system hostname.
	name, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hostname: %v\n", err)
		return 1
	}

	if _, err := fmt.Println(name); err != nil {
		return 1
	}
	return 0
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: hostname [OPTION]...
Print the system hostname.

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
	_, err := fmt.Fprintf(os.Stdout, "hostname (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
