// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd047-hostname R1.1, R1.2, R2.1, R2.2
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and help messages.
const programName = "hostname"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R2.2: check for --help and --version before anything else.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			if _, err := fmt.Print(helpText); err != nil {
				os.Exit(1)
			}
			return
		case "--version":
			if _, err := fmt.Println("hostname (go-unix-utils) 0.1"); err != nil {
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

	// R1.1, R1.2: print the system hostname followed by a newline and exit 0.
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}
	fmt.Println(hostname)
}

// helpText is the usage message printed by --help.
const helpText = `Usage: hostname [OPTION]
Print the system's host name.

      --help     display this help and exit
      --version  output version information and exit
`
