// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd047-hostname R1.1-R1.2, R2.1-R2.2.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: hostname [OPTION]
Print the system hostname.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `hostname (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	for len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case "--":
			args = args[1:]
			goto done
		default:
			if strings.HasPrefix(args[0], "-") && len(args[0]) > 1 {
				fmt.Fprintf(os.Stderr, "hostname: unrecognized option '%s'\n", args[0])
				fmt.Fprintln(os.Stderr, "Try 'hostname --help' for more information.")
				os.Exit(1)
			}
			goto done
		}
	}
done:

	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "hostname: extra operand '%s'\n", args[0])
		fmt.Fprintln(os.Stderr, "Try 'hostname --help' for more information.")
		os.Exit(1)
	}

	name, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hostname: %v\n", err)
		os.Exit(1)
	}

	if _, err := fmt.Fprintln(os.Stdout, name); err != nil {
		os.Exit(1)
	}
}
