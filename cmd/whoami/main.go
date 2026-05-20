// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd042-whoami R1.1, R1.2, R2.1, R2.2.
package main

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: whoami [OPTION]...
Print the user name associated with the current effective user ID.
Same as id -un.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `whoami (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	args = parseFlags(args)

	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "whoami: extra operand '%s'\n", args[0])
		fmt.Fprintln(os.Stderr, "Try 'whoami --help' for more information.")
		os.Exit(1)
	}

	u, err := user.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "whoami: cannot find name for user ID %d: %v\n", os.Geteuid(), err)
		os.Exit(1)
	}

	if _, err := fmt.Fprintln(os.Stdout, u.Username); err != nil {
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
				fmt.Fprintf(os.Stderr, "whoami: unrecognized option '%s'\n", args[0])
				fmt.Fprintln(os.Stderr, "Try 'whoami --help' for more information.")
				os.Exit(1)
			}
			return args
		}
	}
	return args
}
