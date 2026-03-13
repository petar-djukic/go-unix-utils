// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd014-false R1.1–R1.3, R2.1–R2.3, R3.1–R3.2, R4.1–R4.2
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	// R2.1, R2.2: only the first argument is checked for --help/--version.
	// R1.2: all other arguments are ignored.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			// R2.1: print usage to stdout.
			// R3.1: false always exits 1, even after --help output.
			fmt.Print(helpText) // best-effort; exit 1 regardless
		case "--version":
			// R2.2: print version to stdout.
			// R3.1: false always exits 1, even after --version output.
			fmt.Println("false (go-unix-utils) 0.1") // best-effort; exit 1 regardless
		}
	}

	// R1.1: exit 1 unconditionally.
	// R1.3: no reads from stdin, no writes to stdout or stderr.
	os.Exit(1)
}

// helpText is the usage message printed by --help.
const helpText = `Usage: false [ignored command line arguments]
  or:  false OPTION
Exit with a status code indicating failure.

      --help     display this help and exit
      --version  output version information and exit
`
