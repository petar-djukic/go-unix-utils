// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/false: exit with status 1 unconditionally.
// Implements prd014-false R1, R2, R3.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	binName = "false"
	version = "0.1.0"
)

func main() {
	sys.InstallSIGPIPEHandler()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			// R2.1: print usage to stdout, exit 0.
			// R2.3: exit 1 on write error.
			if _, err := fmt.Printf("Usage: %s [ignored command line arguments]\n  or:  %s OPTION\nExit with a status code indicating failure.\n\n      --help        display this help and exit\n      --version     output version information and exit\n", binName, binName); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		case "--version":
			// R2.2: print version to stdout, exit 0.
			if _, err := fmt.Printf("%s (go-unix-utils) %s\n", binName, version); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	// R1.1, R1.2: exit 1 unconditionally, ignoring all arguments.
	os.Exit(1)
}
