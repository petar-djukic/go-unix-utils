// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/false: exit unsuccessfully.
// Implements srd014-false R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R3.1, R3.2.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// versionText is the version string printed when --version is the first argument.
// R2.2: print version information to stdout and exit 0.
const versionText = "false (go-unix-utils)"

// helpText is the usage message printed when --help is the first argument.
// R2.1: print usage to stdout and exit 0.
const helpText = `Usage: false [ignored command line arguments]
  or:  false OPTION
Exit with a status code indicating failure.

      --help        display this help and exit
      --version     output version information and exit
`

func main() {
	sys.InstallSIGPIPEHandler()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			// R2.1: print usage to stdout. GNU gfalse still exits 1.
			fmt.Fprint(os.Stdout, helpText) //nolint:errcheck // best-effort output; exit 1 regardless
		case "--version":
			// R2.2: print version information to stdout. GNU gfalse still exits 1.
			fmt.Fprintln(os.Stdout, versionText) //nolint:errcheck // best-effort output; exit 1 regardless
		}
	}

	// R1.1: exit 1 with no arguments.
	// R1.2: exit 1 with any arguments, ignoring them.
	// R1.3: no stdin read, no stdout/stderr output.
	// R3.1, R3.2: exit is always 1 (or 0 for --help/--version).
	os.Exit(1)
}
