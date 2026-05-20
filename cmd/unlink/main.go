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

const helpText = `Usage: unlink FILE
  or:  unlink OPTION
Call the unlink function to remove the specified FILE.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `unlink (go-unix-utils) dev
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
		fmt.Fprintf(os.Stderr, "unlink: missing operand\n")
		fmt.Fprintf(os.Stderr, "Try 'unlink --help' for more information.\n")
		os.Exit(1)
	}

	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "unlink: extra operand '%s'\n", args[1])
		fmt.Fprintf(os.Stderr, "Try 'unlink --help' for more information.\n")
		os.Exit(1)
	}

	if err := syscall.Unlink(args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "unlink: cannot unlink '%s': %s\n", args[0], capitalizeErr(err))
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
