// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd014-false R1.1–R1.3 (exit 1, ignore arguments, no I/O),
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
			// R2.1, R2.3: print usage; exit 1 regardless (GNU false always exits 1).
			_, _ = fmt.Fprintln(os.Stdout, "Usage: false [ignored command line arguments]")
			_, _ = fmt.Fprintln(os.Stdout, "Exit with a status code indicating failure.")
			os.Exit(1)
		case "--version":
			// R2.2, R2.3: print version; exit 1 regardless (GNU false always exits 1).
			_, _ = fmt.Fprintf(os.Stdout, "false %s\n", version)
			os.Exit(1)
		}
	}

	// R1.1, R1.2, R1.3: ignore all arguments, produce no output, exit 1.
	os.Exit(1)
}
