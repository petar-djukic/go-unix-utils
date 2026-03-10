// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd013-true R1.1–R1.3 (exit 0, ignore arguments, no I/O),
// R2.1–R2.3 (--help and --version output), R3.1–R3.2 (exit codes).
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

	// R2.1: --help as first argument prints usage and exits 0.
	// R2.2: --version as first argument prints version and exits 0.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			// R2.1, R2.3: print usage; exit 1 on write error.
			if _, err := fmt.Fprintln(os.Stdout, "Usage: true [ignored command line arguments]"); err != nil {
				os.Exit(1)
			}
			if _, err := fmt.Fprintln(os.Stdout, "Exit with a status code indicating success."); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		case "--version":
			// R2.2, R2.3: print version; exit 1 on write error.
			if _, err := fmt.Fprintf(os.Stdout, "true %s\n", version); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	// R1.1, R1.2, R1.3: ignore all arguments, produce no output, exit 0.
	os.Exit(0)
}
