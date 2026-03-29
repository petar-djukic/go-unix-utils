// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/true implements GNU true: exit with a status code indicating success.
//
// Implements prd013-true R1.1, R1.2, R1.3, R2.1.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// helpText is the usage message printed when --help is the first argument (R2.1).
const helpText = `Usage: true [ignored command line arguments]
  or:  true OPTION
Exit with a status code indicating success.

      --help     display this help and exit
      --version  output version information and exit
`

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout)
	os.Exit(exitCode)
}

// run implements the true logic: exit 0, optionally printing help.
// R1.1, R1.2: exits 0 unconditionally.
// R1.3: no output unless --help is the first argument.
// R2.1: --help prints usage to stdout and exits 0.
func run(args []string, stdout *os.File) int {
	if len(args) > 0 && args[0] == "--help" {
		if _, err := fmt.Fprint(stdout, helpText); err != nil {
			return 1
		}
	}
	return 0
}
