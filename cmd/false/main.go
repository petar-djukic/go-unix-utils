// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/false exits with status 1, ignoring all arguments.
// Implements srd014-false R1.1-R1.3, R2.1-R2.3, R3.1-R3.2.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: false [ignored command line arguments]
  or:  false OPTION
Exit with a status code indicating failure.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `false (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(1)
		case "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(1)
		}
	}

	os.Exit(1)
}
