// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd042-whoami R1.1, R1.2, R2.1, R2.2
package main

import (
	"fmt"
	"os"
	"os/user"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and version messages.
const programName = "whoami"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R2.2: check for --help and --version before processing.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Print(helpText) //nolint:errcheck
			return
		case "--version":
			fmt.Println("whoami (go-unix-utils) 0.1") //nolint:errcheck
			return
		}
	}

	// R2.1, R2.2: reject unknown flags and extra operands.
	if len(args) > 0 {
		if len(args[0]) > 1 && args[0][0] == '-' {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, args[0])
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		} else {
			fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", programName, args[0])
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		}
		os.Exit(1)
	}

	// R1.1, R1.2: print effective username via os/user.Current().
	u, err := user.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot find name for user ID %d\n", programName, os.Geteuid())
		os.Exit(1)
	}
	fmt.Println(u.Username)
}

// helpText is the usage message printed by --help.
const helpText = `Usage: whoami [OPTION]...
Print the user name associated with the current effective user ID.
Same as id -un.

      --help     display this help and exit
      --version  output version information and exit
`
