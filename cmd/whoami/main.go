// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/whoami implements GNU whoami: print the effective user name.
//
// Implements prd042-whoami R1.1, R1.2, R2.1, R2.2, R3.1, R3.2, R3.3.
package main

import (
	"fmt"
	"os"
	"os/user"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "whoami"

const helpText = `Usage: whoami [OPTION]...
Print the user name associated with the current effective user ID.
Same as id -un.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = "whoami (go-unix-utils) 0.1\n"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// parseResult signals how argument parsing concluded.
type parseResult int

const (
	parseOK           parseResult = iota
	parseHelp                     // --help requested
	parseVer                      // --version requested
	parseErr                      // unknown flag
	parseExtraOperand             // extra operand provided
)

// run parses arguments and prints the effective username.
// Returns exit code: 0 on success, 1 on error.
func run(args []string, stdout, stderr *os.File) int {
	result, badArg := parseArgs(args)
	switch result {
	case parseHelp:
		fmt.Fprint(stdout, helpText) //nolint:errcheck // best-effort
		return 0
	case parseVer:
		fmt.Fprint(stdout, versionText) //nolint:errcheck // best-effort
		return 0
	case parseErr:
		printUnknownOption(badArg, stderr)
		return 1
	case parseExtraOperand:
		printExtraOperand(badArg, stderr)
		return 1
	}

	return printUsername(stdout, stderr)
}

// parseArgs classifies the argument list.
// R2.1: extra operands produce parseExtraOperand.
// R2.2: unknown flags produce parseErr.
func parseArgs(args []string) (parseResult, string) {
	for _, arg := range args {
		switch arg {
		case "--help":
			return parseHelp, ""
		case "--version":
			return parseVer, ""
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return parseErr, arg
			}
			return parseExtraOperand, arg
		}
	}
	return parseOK, ""
}

// printUnknownOption writes the unknown-option error to stderr.
func printUnknownOption(flag string, stderr *os.File) {
	fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", programName, flag) //nolint:errcheck
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", programName) //nolint:errcheck
}

// printExtraOperand writes the extra-operand error to stderr.
func printExtraOperand(operand string, stderr *os.File) {
	fmt.Fprintf(stderr, "%s: extra operand '%s'\n", programName, operand) //nolint:errcheck
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", programName) //nolint:errcheck
}

// printUsername looks up and prints the effective username.
// R1.1: prints the effective username followed by a newline.
// R1.2: uses os/user.Current() which reflects the effective UID.
func printUsername(stdout, stderr *os.File) int {
	u, err := user.Current()
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot find name for user ID: %v\n", programName, err) //nolint:errcheck
		return 1
	}
	fmt.Fprintln(stdout, u.Username) //nolint:errcheck // best-effort
	return 0
}
