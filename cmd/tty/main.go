// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd052-tty R1.1-R2.2.
package main

/*
#include <unistd.h>
*/
import "C"

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: tty [OPTION]...
Print the file name of the terminal connected to standard input.

  -s, --silent, --quiet   print nothing, only return an exit status
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `tty (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()

	silent := parseArgs(os.Args[1:])

	isTTY := sys.IsTerminal(os.Stdin.Fd())

	if !silent {
		if isTTY {
			name := C.ttyname(C.int(os.Stdin.Fd()))
			if name == nil {
				fmt.Fprintln(os.Stdout, "not a tty")
			} else if _, err := fmt.Fprintln(os.Stdout, C.GoString(name)); err != nil {
				os.Exit(1)
			}
		} else {
			fmt.Fprintln(os.Stdout, "not a tty")
		}
	}

	if !isTTY {
		os.Exit(1)
	}
}

func parseArgs(args []string) bool {
	silent := false
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags {
			fmt.Fprintf(os.Stderr, "tty: extra operand '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'tty --help' for more information.")
			os.Exit(2)
		}
		switch {
		case arg == "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case arg == "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case arg == "--silent", arg == "--quiet":
			silent = true
		case arg == "--":
			endOfFlags = true
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr, "tty: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'tty --help' for more information.")
			os.Exit(2)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			silent = parseShortFlags(arg[1:], silent)
		default:
			fmt.Fprintf(os.Stderr, "tty: extra operand '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'tty --help' for more information.")
			os.Exit(2)
		}
	}
	return silent
}

func parseShortFlags(flags string, silent bool) bool {
	for _, ch := range flags {
		switch ch {
		case 's':
			silent = true
		default:
			fmt.Fprintf(os.Stderr, "tty: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'tty --help' for more information.")
			os.Exit(2)
		}
	}
	return silent
}
