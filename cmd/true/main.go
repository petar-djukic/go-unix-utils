// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/true exits with status 0, ignoring all arguments.
// Implements srd013-true R1.1-R1.3, R2.1-R2.3, R3.1-R3.2.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: true [ignored command line arguments]
  or:  true OPTION
Exit with a status code indicating success.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `true (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			if _, err := fmt.Fprint(os.Stdout, helpText); err != nil {
				os.Exit(1)
			}
			return
		case "--version":
			if _, err := fmt.Fprint(os.Stdout, versionText); err != nil {
				os.Exit(1)
			}
			return
		}
	}
}
