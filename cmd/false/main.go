// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the false utility that exits unsuccessfully.
//
// Implements prd014-false: core exit behavior (R1), help and version output (R2),
// exit codes (R3).
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const version = "false (go-unix-utils) 1.0"

func main() {
	sys.InstallSIGPIPEHandler()

	// R2.1, R2.2: --help and --version print output but still exit 1,
	// matching GNU coreutils false behavior.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			fmt.Fprintln(os.Stdout, "Usage: false [ignored command line arguments]\n  or:  false OPTION\nExit with a status code indicating failure.\n\n      --help     display this help and exit\n      --version  output version information and exit")
			os.Exit(1)
		case "--version":
			fmt.Fprintln(os.Stdout, version)
			os.Exit(1)
		}
	}

	// R1.1, R1.2: Exit 1 unconditionally, ignoring all arguments.
	os.Exit(1)
}
