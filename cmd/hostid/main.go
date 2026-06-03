// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd048-hostid R1.1-R1.2, R2.1-R2.2.
package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: hostid [OPTION]
Print the numeric identifier (as a hexadecimal number) for the current host.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `hostid (go-unix-utils) dev
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
				fmt.Fprintf(os.Stderr, "hostid: unrecognized option '%s'\n", args[0])
				fmt.Fprintln(os.Stderr, "Try 'hostid --help' for more information.")
				os.Exit(1)
			}
			goto done
		}
	}
done:

	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "hostid: extra operand '%s'\n", args[0])
		fmt.Fprintln(os.Stderr, "Try 'hostid --help' for more information.")
		os.Exit(1)
	}

	hostid, err := syscall.SysctlUint32("kern.hostid")
	if err != nil {
		fmt.Fprintf(os.Stderr, "hostid: %v\n", err)
		os.Exit(1)
	}

	if _, err := fmt.Fprintf(os.Stdout, "%08x\n", hostid); err != nil {
		os.Exit(1)
	}
}
