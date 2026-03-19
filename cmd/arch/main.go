// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd045-arch R1.1, R1.2 (print machine hardware name),
// R2.1 (extra operand error), R2.2 (unknown flag error).
package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is used in error messages.
const programName = "arch"

// helpText is the usage message printed for --help.
const helpText = `Usage: arch [OPTION]...
Print machine hardware name.

      --help        display this help and exit
      --version     output version information and exit
`

// versionText is the version message printed for --version.
const versionText = "arch (go-unix-utils) 1.0\n"

func main() {
	sys.InstallSIGPIPEHandler()
	run(os.Args[1:])
}

// run processes arguments and prints the machine hardware name.
func run(args []string) {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--help" {
			fmt.Print(helpText)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Print(versionText)
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "-") {
			// R2.2: unknown flag.
			printError(fmt.Sprintf("unrecognized option '%s'", arg))
			os.Exit(1)
		}
		// R2.1: extra operand.
		printError(fmt.Sprintf("extra operand '%s'", arg))
		os.Exit(1)
	}
	// R1.1, R1.2: print machine hardware name.
	name, err := machineName()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}
	fmt.Println(name)
}

// machineName returns the machine hardware name via uname(2).
func machineName() (string, error) {
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		return "", fmt.Errorf("cannot determine architecture: %w", err)
	}
	return unix.ByteSliceToString(utsname.Machine[:]), nil
}

// printError writes a formatted error message to stderr.
func printError(msg string) {
	fmt.Fprintf(os.Stderr,
		"%s: %s\nTry '%s --help' for more information.\n",
		programName, msg, programName)
}
