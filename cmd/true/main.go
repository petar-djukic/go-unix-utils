// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/true: exit successfully.
// Implements srd013-true R1.1, R1.2, R1.3, R2.1.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

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

	// R2.1: when --help is the first argument, print usage and exit 0.
	if len(os.Args) > 1 && os.Args[1] == "--help" {
		if _, err := fmt.Fprint(os.Stdout, helpText); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// R1.1: exit 0 with no arguments.
	// R1.2: exit 0 with any arguments, ignoring them.
	// R1.3: no stdin read, no stdout/stderr output.
	os.Exit(0)
}
