// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd014-false R1.1–R1.3 (exit 1, ignore arguments, no I/O),
// R2.1 (--help and --version output), R2.2 (unknown flags exit 1, no error message),
// R2.3 (operands ignored, exit 1), R3.1–R3.2 (differential tests in false_test.go).
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is injected at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	// R3.1, ARCHITECTURE shared_protocols: exit 0 on SIGPIPE.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R2.1: --help as first argument prints usage and exits 1 (false always exits 1).
	// R2.2: all other flags, including unrecognized ones, are silently ignored; exit 1.
	// R2.3: operand arguments are silently ignored; exit 1.
	// GNU false checks only the first argument for --help and --version, then exits 1.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			// R2.1: print usage to stdout; false exits 1 regardless of flags.
			if _, err := fmt.Fprintln(os.Stdout, "Usage: false [ignored command line arguments]"); err != nil {
				os.Exit(1)
			}
			if _, err := fmt.Fprintln(os.Stdout, "Exit with a status code indicating failure."); err != nil {
				os.Exit(1)
			}
			os.Exit(1)
		case "--version":
			// R2.1: print version to stdout; false exits 1 regardless of flags.
			if _, err := fmt.Fprintf(os.Stdout, "false %s\n", version); err != nil {
				os.Exit(1)
			}
			os.Exit(1)
		default:
			// R2.2, R2.3: unrecognized flags and operands are silently ignored.
			// GNU false exits 1 without printing any error message, matching POSIX
			// semantics where false always fails.
		}
	}

	// R1.1, R1.2, R1.3: exit 1 regardless of arguments; produce no output.
	os.Exit(1)
}
