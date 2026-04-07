// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/true: exit successfully.
// Implements srd013-true R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R3.1, R3.2, R4.1, R4.2, R4.3.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// versionText is the version string printed when --version is the first argument.
// R2.2: print version information to stdout and exit 0.
const versionText = "true (go-unix-utils)"

// helpText is the usage message printed when --help is the first argument.
// R2.1: print usage to stdout and exit 0.
const helpText = `Usage: true [ignored command line arguments]
  or:  true OPTION
Exit with a status code indicating success.

      --help        display this help and exit
      --version     output version information and exit
`

func main() {
	sys.InstallSIGPIPEHandler()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			// R2.1: print usage to stdout and exit 0.
			// R2.3: exit 1 on write error.
			if _, err := fmt.Fprint(os.Stdout, helpText); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		case "--version":
			// R2.2: print version information to stdout and exit 0.
			// R2.3: exit 1 on write error.
			if _, err := fmt.Fprintln(os.Stdout, versionText); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	// R1.1: exit 0 with no arguments.
	// R1.2: exit 0 with any arguments, ignoring them.
	// R1.3: no stdin read, no stdout/stderr output.
	// R3.1, R3.2: exit is always 0 (or 1 on write error above).
	os.Exit(0)
}
