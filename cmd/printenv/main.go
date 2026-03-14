// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd040-printenv R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and --help output.
const programName = "printenv"

// exitNotFound is the exit code when any requested variable is not set.
const exitNotFound = 1

// exitUsageError is the exit code for usage errors.
const exitUsageError = 2

func main() {
	// D1: Install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// D3: Parse flags manually with long-flag aliases.
	nullTerminate := false
	var varNames []string

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--help" {
			printHelp()
			return
		}

		if arg == "--version" {
			printVersion()
			return
		}

		// R2.1: -0 / --null terminates output with NUL instead of newline.
		if arg == "-0" || arg == "--null" {
			nullTerminate = true
			i++
			continue
		}

		// End-of-options marker.
		if arg == "--" {
			i++
			break
		}

		// Reject unknown long flags.
		if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(exitUsageError)
		}

		// Reject unknown short flags.
		if strings.HasPrefix(arg, "-") && arg != "-" {
			fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, arg[1])
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(exitUsageError)
		}

		// Positional argument: variable name.
		break
	}

	// Remaining arguments are variable names.
	if i < len(args) {
		varNames = args[i:]
	}

	// D4: Use buffered writer for efficient I/O.
	w := bufio.NewWriter(os.Stdout)

	terminator := "\n"
	if nullTerminate {
		terminator = "\x00"
	}

	// R1.1 / R2.4: No variable arguments — print all environment variables and exit 0.
	if len(varNames) == 0 {
		for _, kv := range os.Environ() {
			fmt.Fprint(w, kv+terminator)
		}
		w.Flush()
		return
	}

	// R1.2 / R1.3 / R2.2 / R2.3: Print values of named variables.
	allFound := true
	for _, name := range varNames {
		val, ok := os.LookupEnv(name)
		if !ok {
			allFound = false
			continue
		}
		fmt.Fprint(w, val+terminator)
	}
	w.Flush()

	if !allFound {
		os.Exit(exitNotFound)
	}
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: printenv [OPTION]... [VARIABLE]...
Print the values of the specified environment VARIABLE(s).
If no VARIABLE is specified, print name and value pairs for them all.

  -0, --null     end each output line with NUL, not newline
      --help     display this help and exit
      --version  output version information and exit
`)
}

// printVersion writes version information to stdout.
func printVersion() {
	fmt.Println("printenv (go-unix-utils) 0.1")
}
