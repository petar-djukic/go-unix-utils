// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd014-false: Exit Unsuccessfully.
// Covers R1.1-R1.3 (core exit behavior), R2.1-R2.3 (help/version output),
// R3.1-R3.2 (exit codes).
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// Defaults to "dev" when the linker variable is not set.
var version = "dev"

func main() {
	// R3.1: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	// R2.1-R2.3: check for --help and --version as first argument.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			os.Exit(printHelp())
		case "--version":
			os.Exit(printVersion())
		}
	}

	// R1.1-R1.3: exit 1 unconditionally, ignoring all arguments.
	os.Exit(1)
}

// printHelp writes usage information to stdout and returns the exit code.
// R2.1: prints help to stdout. R2.3: exits 1 on write error.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: false [ignored command line arguments]
  or:  false OPTION
Exit with a status code indicating failure.

      --help     display this help and exit
      --version  output version information and exit

NOTE: your shell may have its own version of false, which usually supersedes
the version described here.  Please refer to your shell's documentation
for details about the options it supports.
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
// R2.2: prints version to stdout. R2.3: exits 1 on write error.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "false (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
