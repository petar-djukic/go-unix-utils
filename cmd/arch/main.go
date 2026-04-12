// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/arch: print machine hardware name.
// Implements srd045-arch R1.1, R1.2, R2.1, R2.2.
package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "arch"

// versionText is printed when --version is passed.
// R2.2: print version information to stdout and exit 0.
const versionText = progName + " (go-unix-utils)"

// helpText is the usage message printed when --help is passed.
// R2.1: print usage to stdout and exit 0.
const helpText = `Usage: arch [OPTION]...
Print machine hardware name.

      --help        display this help and exit
      --version     output version information and exit
`

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	if len(args) > 0 {
		if handled := handleInfoFlags(args); handled {
			return
		}
		// R2.1, R2.2: reject extra operands and unknown flags.
		printExtraOperandError(args[0])
		os.Exit(1)
	}

	// R1.1: print machine hardware name followed by a newline.
	machine := getMachineHardwareName()
	fmt.Println(machine)
}

// handleInfoFlags checks for --help and --version as the first argument.
// Returns true if a flag was handled (caller should return).
func handleInfoFlags(args []string) bool {
	switch args[0] {
	case "--help":
		fmt.Print(helpText)
		return true
	case "--version":
		fmt.Println(versionText)
		return true
	}
	return false
}

// printExtraOperandError prints the GNU-style error for extra operands or
// unknown flags.
func printExtraOperandError(arg string) {
	fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, arg)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
}

// getMachineHardwareName returns the machine hardware name from uname(2).
// R1.2: output must be identical to guname -m.
func getMachineHardwareName() string {
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot determine machine hardware name: %v\n", progName, err)
		os.Exit(1)
	}
	return utsnameBytesToString(utsname.Machine[:])
}

// utsnameBytesToString converts a null-terminated byte array to a Go string.
func utsnameBytesToString(raw []byte) string {
	for i, v := range raw {
		if v == 0 {
			return string(raw[:i])
		}
	}
	return string(raw)
}
