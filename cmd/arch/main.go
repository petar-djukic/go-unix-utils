// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/arch prints the machine hardware name, equivalent to uname -m.
// Implements prd045-arch R1.1, R1.2, R2.1, R2.2.
package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error messages.
const programName = "arch"

func main() {
	sys.InstallSIGPIPEHandler()

	// R2.1, R2.2: reject any arguments (flags or operands).
	if len(os.Args) > 1 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", programName, os.Args[1])
		os.Exit(1)
	}

	// R1.1, R1.2: print machine hardware name matching uname -m / garch.
	machine, err := syscall.Sysctl("hw.machine")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}
	fmt.Println(machine)
}
