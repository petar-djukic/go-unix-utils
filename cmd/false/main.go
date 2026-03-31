// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/false implements GNU false: exit with a status code indicating failure.
//
// Implements prd014-false R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R3.1, R3.2.
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
const helpText = `Usage: false [ignored command line arguments]
  or:  false OPTION
Exit with a status code indicating failure.

      --help     display this help and exit
      --version  output version information and exit
`

func main() {
	// R3.1: install SIGPIPE handler for clean exit on broken pipe.
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout)
	os.Exit(exitCode)
}

// run implements the false logic: always exit 1, optionally printing help or version.
// R1.1: exits 1 with no arguments.
// R1.2, R1.3: exits 1 with any arguments, ignoring them.
// R2.1: --help as first argument prints usage to stdout. Exits 1 per GNU gfalse behavior.
// R2.2: --version as first argument prints version info to stdout. Exits 1.
// R3.1, R3.2: always exits 1 (GNU false never exits 0, unlike true).
func run(args []string, stdout *os.File) int {
	if len(args) == 0 {
		return 1
	}
	switch args[0] {
	case "--help":
		// R2.1: print help. GNU gfalse always exits 1 even for --help.
		fmt.Fprint(stdout, helpText) //nolint:errcheck // best-effort output, exit 1 regardless
	case "--version":
		// R2.2: print version info. GNU gfalse always exits 1 even for --version.
		fmt.Fprintf(stdout, "false (go-unix-utils) %s\n", Version) //nolint:errcheck // best-effort output, exit 1 regardless
	}
	return 1
}
