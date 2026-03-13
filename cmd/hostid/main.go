// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd048-hostid R1.1, R1.2, R2.1, R2.2
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

// programName is the name used in error and help messages.
const programName = "hostid"

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
			if _, err := fmt.Println("hostid (go-unix-utils) 0.1"); err != nil {
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

	// R1.1, R1.2: print the 32-bit host identifier as an 8-digit zero-padded
	// lowercase hexadecimal number followed by a newline, derived from gethostid(3).
	hostID := C.gethostid()
	fmt.Printf("%08x\n", uint32(hostID))
}

// helpText is the usage message printed by --help.
const helpText = `Usage: hostid [OPTION]
Print the numeric identifier (as a hexadecimal number) for the current host.

      --help     display this help and exit
      --version  output version information and exit
`
