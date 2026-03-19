// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd014-false R1.1-R1.3: core false behavior (exit 1, no output),
// R2.1-R2.3: --help and --version flag handling,
// R3.1-R3.2: exit code contracts (1 default, 0 on --help/--version success).
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// helpText is the usage message printed for --help.
const helpText = `Usage: false [ignored command line arguments]
  or:  false OPTION
Exit with a status code indicating failure.

      --help        display this help and exit
      --version     output version information and exit

NOTE: your shell may have its own version of false, which usually supersedes
the version described here.  Please refer to your shell's documentation
for details about the options it supports.
`

// versionText is the version message printed for --version.
const versionText = `false (go-unix-utils) 1.0
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

	// R1.1-R1.3, R2.3: exit 1, no output, ignore all arguments.
	os.Exit(1)
}

// printAndExit writes text to stdout and exits. GNU false exits 1 even for
// --help and --version, unlike GNU true which exits 0.
// R2.3: write error during --help or --version output exits 1.
func printAndExit(text string) {
	fmt.Fprint(os.Stdout, text) //nolint:errcheck // R2.3: exit 1 regardless
	os.Exit(1)
}
