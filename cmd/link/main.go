// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: link FILE1 FILE2
  or:  link OPTION
Call the link function to create a link named FILE2 to an existing FILE1.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `link (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	for _, arg := range args {
		switch arg {
		case "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		}
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "link: missing operand\n")
		fmt.Fprintf(os.Stderr, "Try 'link --help' for more information.\n")
		os.Exit(1)
	}

	if len(args) == 1 {
		fmt.Fprintf(os.Stderr, "link: missing operand after '%s'\n", args[0])
		fmt.Fprintf(os.Stderr, "Try 'link --help' for more information.\n")
		os.Exit(1)
	}

	if len(args) > 2 {
		fmt.Fprintf(os.Stderr, "link: extra operand '%s'\n", args[2])
		fmt.Fprintf(os.Stderr, "Try 'link --help' for more information.\n")
		os.Exit(1)
	}

	if err := syscall.Link(args[0], args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "link: cannot create link '%s' to '%s': %s\n", args[1], args[0], capitalizeErr(err))
		os.Exit(1)
	}
}

func capitalizeErr(err error) string {
	msg := err.Error()
	if len(msg) > 0 {
		return strings.ToUpper(msg[:1]) + msg[1:]
	}
	return msg
}
