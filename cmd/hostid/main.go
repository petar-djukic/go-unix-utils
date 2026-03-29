// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/hostid prints the 32-bit host identifier as an 8-digit lowercase
// hexadecimal number.
// Implements prd048-hostid R1.1, R1.2, R2.1, R2.2.
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

// programName is the name used in error messages.
const programName = "hostid"

func main() {
	sys.InstallSIGPIPEHandler()

	// R2.2: reject unknown flags.
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-") {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		}
	}

	// R2.1: reject any extra operands.
	if len(os.Args) > 1 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", programName, os.Args[1])
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	// R1.1, R1.2: print host identifier from gethostid(3).
	id := uint32(C.gethostid())
	fmt.Printf("%08x\n", id)
}
