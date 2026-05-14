// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd045-arch R1.1-R1.2, R2.1-R2.2.
package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: arch [OPTION]...
Print machine architecture.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `arch (go-unix-utils) dev
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
				fmt.Fprintf(os.Stderr, "arch: unrecognized option '%s'\n", args[0])
				fmt.Fprintln(os.Stderr, "Try 'arch --help' for more information.")
				os.Exit(1)
			}
			goto done
		}
	}
done:

	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "arch: extra operand '%s'\n", args[0])
		fmt.Fprintln(os.Stderr, "Try 'arch --help' for more information.")
		os.Exit(1)
	}

	machine, err := machineHardwareName()
	if err != nil {
		fmt.Fprintf(os.Stderr, "arch: %v\n", err)
		os.Exit(1)
	}

	if _, err := fmt.Fprintln(os.Stdout, machine); err != nil {
		os.Exit(1)
	}
}

func machineHardwareName() (string, error) {
	return syscall.Sysctl("hw.machine")
}
