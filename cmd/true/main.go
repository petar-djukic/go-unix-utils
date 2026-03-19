// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd013-true R1.1-R1.3: core true behavior (exit 0, no output),
// R2.1-R2.3: --help and --version flag handling,
// R3.1-R3.2: exit code contracts (0 on success, 1 on write error only).
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// helpText is the usage message printed for --help.
const helpText = `Usage: true [ignored command line arguments]
  or:  true OPTION
Exit with a status code indicating success.

      --help        display this help and exit
      --version     output version information and exit

NOTE: your shell may have its own version of true, which usually supersedes
the version described here.  Please refer to your shell's documentation
for details about the options it supports.
`

// versionText is the version message printed for --version.
const versionText = `true (go-unix-utils) 1.0
`

func main() {
	sys.InstallSIGPIPEHandler()

	// R2.1-R2.2: check first argument for --help or --version.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			printAndExit(helpText)
		case "--version":
			printAndExit(versionText)
		}
	}

	// R1.1-R1.3: exit 0, no output, ignore all arguments.
}

// printAndExit writes text to stdout and exits 0 on success or 1 on write error.
// R2.3: write error during --help or --version output exits 1.
func printAndExit(text string) {
	_, err := fmt.Fprint(os.Stdout, text)
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
