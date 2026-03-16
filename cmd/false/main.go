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
			// R2.1: print usage message to stdout.
			// GNU false always exits 1, even for --help/--version.
			fmt.Fprintf(os.Stdout, //nolint:errcheck // write error does not change exit code
				"Usage: %s [ignored command line arguments]\n  or:  %s OPTION\nExit with a status code indicating failure.\n\n      --help     display this help and exit\n      --version  output version information and exit\n",
				progName, progName,
			)
			// R3.1: false always exits 1.
			os.Exit(1)
		case "--version":
			// R2.2: print version information to stdout.
			// GNU false always exits 1, even for --help/--version.
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n", //nolint:errcheck // write error does not change exit code
				progName, "go-unix-utils", version.Version,
			)
			// R3.1: false always exits 1.
			os.Exit(1)
		}
	}

	// R1.1, R1.2: exit 1 unconditionally, ignoring all arguments.
	// R3.1: default exit is always 1.
	os.Exit(1)
}
