// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/true implements GNU true: exit with a status code indicating success.
//
// Implements prd013-true R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R3.1, R3.2.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// Version is set at build time via -ldflags "-X main.Version=<tag>".
// Defaults to "dev" for development builds.
var Version = "dev"

// helpText is the usage message printed when --help is the first argument (R2.1).
const helpText = `Usage: true [ignored command line arguments]
  or:  true OPTION
Exit with a status code indicating success.

      --help     display this help and exit
      --version  output version information and exit
`

func main() {
	// R3.2: install SIGPIPE handler for clean exit on broken pipe.
	sys.InstallSIGPIPEHandler()

	run(os.Args[1:], os.Stdout)
	os.Exit(0)
}

// run implements the true logic: exit 0, optionally printing help or version.
// R1.1, R1.2: exits 0 unconditionally.
// R1.3: no output unless --help or --version is the first argument.
// R2.1: --help prints usage to stdout.
// R2.2: --version prints version info to stdout.
// R2.3, R3.1: write errors are suppressed — true always exits 0.
func run(args []string, stdout *os.File) {
	if len(args) == 0 {
		return
	}
	switch args[0] {
	case "--help":
		// R2.1: print help. Write error suppressed per D3.
		fmt.Fprint(stdout, helpText) //nolint:errcheck // R3.1: true always exits 0
	case "--version":
		// R2.2: print version info matching GNU format.
		fmt.Fprintf(stdout, "true (go-unix-utils) %s\n", Version) //nolint:errcheck // R3.1: true always exits 0
	}
}
