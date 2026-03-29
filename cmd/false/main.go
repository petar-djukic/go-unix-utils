// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/false implements GNU false: exit with a status code indicating failure.
//
// Implements prd014-false R1.1, R1.2, R1.3, R2.1.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// helpText is the usage message printed when --help is the first argument (R2.1).
const helpText = `Usage: false [ignored command line arguments]
  or:  false OPTION
Exit with a status code indicating failure.

      --help     display this help and exit
      --version  output version information and exit
`

func main() {
	// R3.2: install SIGPIPE handler for clean exit on broken pipe.
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout)
	os.Exit(exitCode)
}

// run implements the false logic: exit 1, optionally printing help.
// R1.1: exits 1 with no arguments.
// R1.2, R1.3: exits 1 with any arguments, ignoring them.
// R2.1: --help as first argument prints usage to stdout and exits 0.
func run(args []string, stdout *os.File) int {
	if len(args) > 0 && args[0] == "--help" {
		// R2.1: print help and exit 0.
		_, err := fmt.Fprint(stdout, helpText)
		if err != nil {
			return 1
		}
		return 0
	}
	return 1
}
