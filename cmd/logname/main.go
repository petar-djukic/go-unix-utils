// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd053-logname R1.1, R1.2, R2.1, R2.2, R2.3
package main

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and help messages.
const programName = "logname"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R2.1, R2.2: check for --help and --version before anything else.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			if _, err := fmt.Print(helpText); err != nil {
				os.Exit(1)
			}
			return
		case "--version":
			if _, err := fmt.Println("logname (go-unix-utils) 0.1"); err != nil {
				os.Exit(1)
			}
			return
		}
	}

	// R2.1, R2.2: reject any operands or unknown flags.
	if len(args) > 0 {
		arg := args[0]
		if strings.HasPrefix(arg, "-") {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\nTry '%s --help' for more information.\n", programName, arg, programName)
		} else {
			fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\nTry '%s --help' for more information.\n", programName, arg, programName)
		}
		os.Exit(1)
	}

	// R1.1, R1.2: print the login name followed by a newline and exit 0.
	// Use LOGNAME environment variable first (set by the login process),
	// falling back to os/user.Current() which queries the password database.
	loginName := os.Getenv("LOGNAME")
	if loginName == "" {
		// R2.3: fallback to current user; if that also fails, report error.
		u, err := user.Current()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: no login name\n", programName)
			os.Exit(1)
		}
		loginName = u.Username
	}
	fmt.Println(loginName)
}

// helpText is the usage message printed by --help.
const helpText = `Usage: logname [OPTION]
Print the name of the current user.

      --help     display this help and exit
      --version  output version information and exit
`
