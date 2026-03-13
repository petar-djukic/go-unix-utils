// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd014-false R1.1–R1.3, R2.1–R2.3
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
			// R2.1: print usage to stdout and exit 0.
			// R2.3: exit 1 on write error.
			if _, err := fmt.Print(helpText); err != nil {
				os.Exit(1)
			}
			return
		case "--version":
			// R2.2: print version to stdout and exit 0.
			// R2.3: exit 1 on write error.
			if _, err := fmt.Println("false (go-unix-utils) 0.1"); err != nil {
				os.Exit(1)
			}
			return
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
