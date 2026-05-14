// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd053-logname R1.1-R1.2, R2.1-R2.3.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: logname [OPTION]
Print the name of the current user.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `logname (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	args = parseFlags(args)

	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "logname: extra operand '%s'\n", args[0])
		fmt.Fprintln(os.Stderr, "Try 'logname --help' for more information.")
		os.Exit(1)
	}

	loginName := os.Getenv("LOGNAME")
	if loginName == "" {
		fmt.Fprintln(os.Stderr, "logname: no login name")
		os.Exit(1)
	}

	if _, err := fmt.Fprintln(os.Stdout, loginName); err != nil {
		os.Exit(1)
	}
}

func parseFlags(args []string) []string {
	for len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case "--":
			return args[1:]
		default:
			if strings.HasPrefix(args[0], "-") && len(args[0]) > 1 {
				fmt.Fprintf(os.Stderr, "logname: unrecognized option '%s'\n", args[0])
				fmt.Fprintln(os.Stderr, "Try 'logname --help' for more information.")
				os.Exit(1)
			}
			return args
		}
	}
	return args
}
