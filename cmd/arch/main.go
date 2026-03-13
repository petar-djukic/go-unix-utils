// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd045-arch R1.1, R1.2, R2.1, R2.2, R3.1, R3.2
package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and help messages.
const programName = "arch"

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R2.1, R2.2, R3.1, R3.2: check for --help and --version before anything else.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			// R3.2: print usage to stdout and exit 0; exit 1 on write error.
			if _, err := fmt.Print(helpText); err != nil {
				os.Exit(1)
			}
			return
		case "--version":
			// R3.1: print version to stdout and exit 0; exit 1 on write error.
			if _, err := fmt.Println("arch (go-unix-utils) 0.1"); err != nil {
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

	// R1.1: print the machine hardware name followed by a newline.
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}
	fmt.Println(bytesToString(utsname.Machine))
}

// bytesToString converts a null-terminated byte array to a Go string.
func bytesToString(field [256]byte) string {
	for i, b := range field {
		if b == 0 {
			return string(field[:i])
		}
	}
	return string(field[:])
}

// helpText is the usage message printed by --help.
const helpText = `Usage: arch [OPTION]
Print machine architecture.

      --help     display this help and exit
      --version  output version information and exit
`
