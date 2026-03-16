// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd013-true R1.1-R1.3, R2.1-R2.3, R3.1-R3.2:
// cmd/true exits with status 0 unconditionally. Ignores all arguments.
// Supports --help and --version as the first argument. Installs SIGPIPE
// handler for clean exit on broken pipe.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in help output.
const progName = "true"

func main() {
	// R1.3: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	// R2.1, R2.2: check first argument for --help or --version.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			// R2.1: print usage message to stdout and exit 0.
			if _, err := fmt.Fprintf(os.Stdout,
				"Usage: %s [ignored command line arguments]\nExit with a status code indicating success.\n",
				progName,
			); err != nil {
				// R2.3: write error during --help output exits 1.
				os.Exit(1)
			}
			os.Exit(0)
		case "--version":
			// R2.2: print version information to stdout and exit 0.
			if _, err := fmt.Fprintf(os.Stdout, "%s (%s) %s\n",
				progName, "go-unix-utils", version.Version,
			); err != nil {
				// R2.3: write error during --version output exits 1.
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	// R1.1, R1.2: exit 0 unconditionally, ignoring all arguments.
	// R1.3: no stdin read, no stdout/stderr output.
}
