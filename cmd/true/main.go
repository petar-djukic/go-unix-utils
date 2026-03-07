// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the true utility that exits successfully.
//
// Implements prd013-true: core exit behavior (R1), help and version output (R2),
// exit codes (R3).
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const version = "true (go-unix-utils) 1.0"

func main() {
	sys.InstallSIGPIPEHandler()

	// R2.1, R2.2: Only --help and --version as the first argument are recognized.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			_, err := fmt.Fprintln(os.Stdout, "Usage: true [ignored command line arguments]\n  or:  true OPTION\nExit with a status code indicating success.\n\n      --help     display this help and exit\n      --version  output version information and exit")
			if err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		case "--version":
			_, err := fmt.Fprintln(os.Stdout, version)
			if err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	// R1.1, R1.2: Exit 0 unconditionally, ignoring all arguments.
	os.Exit(0)
}
