// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd052-tty R1.1 (print terminal device name),
// R1.2 (exit 0 for terminal, exit 1 otherwise),
// R1.3 (-s/--silent/--quiet suppresses output),
// R2.1 (extra operands produce error exit 2).
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

// programName is used in error messages.
const programName = "tty"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run processes arguments, checks if stdin is a terminal,
// and prints the result. Returns the exit code.
func run(args []string) int {
	silent, err := parseArgs(args)
	if err != nil {
		printError(err.Error())
		return 2
	}
	isTTY := sys.IsTerminal(os.Stdin.Fd())
	if !silent {
		printResult(isTTY)
	}
	if isTTY {
		return 0
	}
	return 1
}

// printResult prints the terminal device name or "not a tty".
// R1.1: prints device name when stdin is a terminal.
// R1.2: prints "not a tty" when stdin is not a terminal.
func printResult(isTTY bool) {
	if !isTTY {
		fmt.Println("not a tty")
		return
	}
	fmt.Println(getTTYName())
}

// getTTYName retrieves the terminal device name for stdin
// via ttyname(3). Falls back to "not a tty" if unavailable.
func getTTYName() string {
	p := C.ttyname(0)
	if p == nil {
		return "not a tty"
	}
	return C.GoString(p)
}

// parseArgs validates command-line arguments and returns
// whether silent mode is enabled.
// R1.3: -s/--silent/--quiet suppresses output.
// R2.1: extra operands produce error exit 2.
func parseArgs(args []string) (bool, error) {
	silent := false
	for i, arg := range args {
		if arg == "--" {
			if i+1 < len(args) {
				return false, fmt.Errorf("extra operand '%s'", args[i+1])
			}
			break
		}
		var err error
		silent, err = handleArg(arg, silent)
		if err != nil {
			return false, err
		}
	}
	return silent, nil
}

// handleArg processes a single argument, returning updated
// silent state or an error for unrecognized input.
func handleArg(arg string, silent bool) (bool, error) {
	switch arg {
	case "--help":
		fmt.Print(helpText())
		os.Exit(0)
	case "--version":
		fmt.Print(versionText())
		os.Exit(0)
	case "-s", "--silent", "--quiet":
		return true, nil
	default:
		return silent, classifyBadArg(arg)
	}
	return silent, nil // unreachable
}

// classifyBadArg returns the appropriate error for an
// unrecognized argument: unknown flag or extra operand.
func classifyBadArg(arg string) error {
	if strings.HasPrefix(arg, "--") {
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	if strings.HasPrefix(arg, "-") && len(arg) > 1 {
		return fmt.Errorf("invalid option -- '%c'", arg[1])
	}
	return fmt.Errorf("extra operand '%s'", arg)
}

// helpText returns the --help output.
func helpText() string {
	return `Usage: tty [OPTION]...
Print the file name of the terminal connected to standard input.

  -s, --silent, --quiet   print nothing, only return an exit status
      --help     display this help and exit
      --version  output version information and exit
`
}

// versionText returns the --version output.
func versionText() string {
	return "tty (go-unix-utils) 1.0\n"
}

// printError writes a formatted error message to stderr.
func printError(msg string) {
	fmt.Fprintf(os.Stderr,
		"%s: %s\nTry '%s --help' for more information.\n",
		programName, msg, programName)
}
