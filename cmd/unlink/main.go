// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd038-unlink: Remove a Single File.
// Covers R1.1-R1.3 (basic unlink operation, exit codes),
// R2.1-R2.4 (error handling for wrong argument count, nonexistent files, directories).
package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const progName = "unlink"

func main() {
	sys.InstallSIGPIPEHandler()

	exit := run(os.Args[1:])
	os.Exit(exit)
}

// run parses arguments and executes the unlink operation. Returns exit code.
func run(args []string) int {
	operands, exitCode := parseArgs(args)
	if exitCode >= 0 {
		return exitCode
	}

	// R2.1: zero arguments is an error.
	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		printTryHelp()
		return 1
	}

	// R2.2: more than one argument is an error.
	if len(operands) > 1 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, operands[1])
		printTryHelp()
		return 1
	}

	// R1.1/R2.4: call unlink(2) directly; rejects directories with EPERM.
	if err := syscall.Unlink(operands[0]); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot unlink '%s': %s\n",
			progName, operands[0], err.Error())
		return 1
	}

	// R1.2: exit 0 on success, no stdout output.
	return 0
}

// parseArgs processes flags and returns operands.
// exit is -1 when processing should continue; >= 0 for early termination.
func parseArgs(args []string) (operands []string, exit int) {
	exit = -1
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			operands = append(operands, args[i+1:]...)
			return
		case arg == "--help":
			return nil, printHelp()
		case arg == "--version":
			return nil, printVersion()
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// R2.3: reject unknown flags.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
			printTryHelp()
			return nil, 1
		default:
			operands = append(operands, args[i:]...)
			return
		}
	}
	return
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: unlink FILE
  or:  unlink OPTION
Call the unlink function to remove the specified FILE.

      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "%s (go-unix-utils) %s\n", progName, version)
	if err != nil {
		return 1
	}
	return 0
}
