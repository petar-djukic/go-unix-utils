// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd014-false R1.1-R1.3, R2.1-R2.3, R3.1-R3.2:
// cmd/false exits with status 1 unconditionally. Ignores all arguments.
// Supports --help and --version as the first argument. Installs SIGPIPE
// handler for clean exit on broken pipe. Write errors on --help/--version
// cause exit 1 per R2.3.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in help and diagnostic output.
const progName = "false"

func main() {
	// R1.3: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	// R2.1, R2.2: check first argument for --help or --version.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			// R2.1: print usage message to stdout. Exit 1 regardless (false always exits 1).
			if _, err := fmt.Fprintf(os.Stdout,
				"Usage: %s [ignored command line arguments]\n  or:  %s OPTION\nExit with a status code indicating failure.\n",
				progName, progName,
			); err != nil {
				// R2.3: report write error to stderr, still exit 1.
				fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err) //nolint:errcheck // best-effort diagnostic
			}
			os.Exit(1)
		case "--version":
			// R2.2: print version information to stdout. Exit 1 regardless (false always exits 1).
			if _, err := fmt.Fprintf(os.Stdout, "%s (%s) %s\n",
				progName, "go-unix-utils", version.Version,
			); err != nil {
				// R2.3: report write error to stderr, still exit 1.
				fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err) //nolint:errcheck // best-effort diagnostic
			}
			os.Exit(1)
		}
	}

	// R1.1, R1.2: exit 1 unconditionally, ignoring all arguments.
	// R3.1: default exit is always 1.
	os.Exit(1)
}
