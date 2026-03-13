// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd034-mkdir R1.1–R1.4, R2.1–R2.3
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and --help output.
const programName = "mkdir"

func main() {
	// D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var operands []string
	// R2.1: -p / --parents flag for recursive directory creation.
	parents := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help":
			printHelp()
			return
		case arg == "--version":
			printVersion()
			return
		case arg == "--parents":
			parents = true
		case arg == "--":
			// End of flags; remaining args are operands.
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "--"):
			// Unrecognized long option.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Parse bundled short options (e.g., -p).
			for _, ch := range arg[1:] {
				switch ch {
				case 'p':
					parents = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, ch)
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
					os.Exit(1)
				}
			}
		default:
			operands = append(operands, arg)
		}
	}

	// R1.1, R1.2: at least one operand is required.
	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	exitCode := 0
	for _, dir := range operands {
		if parents {
			// R2.1: create intermediate parent directories as needed.
			// R2.2: no error when target already exists with -p.
			// R2.3: no error when intermediate directories already exist with -p.
			if err := os.MkdirAll(dir, 0o777); err != nil {
				fmt.Fprintf(os.Stderr, "%s: cannot create directory '%s': %s\n", programName, dir, errMessage(err))
				exitCode = 1
			}
		} else {
			// R1.1: create directory with default permissions (0777 modified by umask).
			// R1.2: process each operand independently.
			// R1.3: os.Mkdir returns an error when the parent does not exist.
			// R1.3 (task R1.2): os.Mkdir returns an error when the target already exists.
			if err := os.Mkdir(dir, 0o777); err != nil {
				fmt.Fprintf(os.Stderr, "%s: cannot create directory '%s': %s\n", programName, dir, errMessage(err))
				exitCode = 1
			}
		}
	}

	os.Exit(exitCode)
}

// errMessage extracts the underlying error message from a *os.PathError,
// stripping the op and path prefix that Go adds, to match GNU coreutils style.
func errMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// printHelp writes usage information to stdout and exits 0.
//
// R1.4: --help prints usage to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: mkdir [OPTION]... DIRECTORY...
Create the DIRECTORY(ies), if they do not already exist.

  -p, --parents  no error if existing, make parent directories as needed
      --help     display this help and exit
      --version  output version information and exit
`)
}

// printVersion writes version information to stdout and exits 0.
//
// R1.4: --version prints version info to stdout and exits 0.
func printVersion() {
	fmt.Println("mkdir (go-unix-utils) 0.1")
}
