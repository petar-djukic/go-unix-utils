// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/logname implements GNU logname: print the login name of the current user.
//
// Implements prd053-logname R1.1, R1.2, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "logname"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and prints the login name. Returns exit code.
// R1.1: prints login name followed by newline and exits 0.
// R2.1: extra operands produce an error and exit 1.
func run(args []string, stdout, stderr *os.File) int {
	result, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err) //nolint:errcheck
		printTryHelp(stderr)
		return 1
	}
	if result == resultHelp {
		printHelp(stdout)
		return 0
	}
	if result == resultVersion {
		printVersion(stdout)
		return 0
	}
	return printLogname(stdout, stderr)
}

// printLogname retrieves and prints the login name.
// R1.2: obtains login name from the system login record.
// R2.3: exits 1 if the login name cannot be determined.
func printLogname(stdout, stderr *os.File) int {
	name, err := getLoginName()
	if err != nil {
		fmt.Fprintf(stderr, "%s: no login name\n", progName) //nolint:errcheck
		return 1
	}
	fmt.Fprintln(stdout, name) //nolint:errcheck
	return 0
}

// getLoginName returns the login name from the system login record.
// R1.2: uses os/user.Current as the Go equivalent of getlogin(3).
func getLoginName() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

// parseResult describes the outcome of argument parsing.
type parseResult int

const (
	resultContinue parseResult = iota
	resultHelp
	resultVersion
)

// parseArgs validates command-line arguments for logname.
// R2.1: extra operands produce an error.
// R2.2: unknown flags produce an error.
func parseArgs(args []string) (parseResult, error) {
	seenDash := false
	for _, arg := range args {
		if seenDash || !strings.HasPrefix(arg, "-") || arg == "-" {
			return resultContinue, fmt.Errorf("extra operand '%s'", arg)
		}
		if arg == "--" {
			seenDash = true
			continue
		}
		result, err := parseFlag(arg)
		if err != nil {
			return resultContinue, err
		}
		if result != resultContinue {
			return result, nil
		}
	}
	return resultContinue, nil
}

// parseFlag parses a single flag argument.
func parseFlag(arg string) (parseResult, error) {
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(arg)
	}
	return resultContinue, parseShortFlags(arg)
}

// parseLongFlag handles --help and --version.
func parseLongFlag(arg string) (parseResult, error) {
	switch arg {
	case "--help":
		return resultHelp, nil
	case "--version":
		return resultVersion, nil
	default:
		return resultContinue, fmt.Errorf("unrecognized option '%s'", arg)
	}
}

// parseShortFlags rejects all short flags as unknown.
func parseShortFlags(arg string) error {
	for _, ch := range arg[1:] {
		return fmt.Errorf("invalid option -- '%c'", ch)
	}
	return nil
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(stderr *os.File) {
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
}

// printHelp writes usage information to stdout.
func printHelp(stdout *os.File) {
	fmt.Fprintf(stdout, "Usage: %s\n", progName)                                    //nolint:errcheck
	fmt.Fprintln(stdout, "Print the name of the current user.")                     //nolint:errcheck
	fmt.Fprintln(stdout)                                                            //nolint:errcheck
	fmt.Fprintln(stdout, "      --help              display this help and exit")    //nolint:errcheck
	fmt.Fprintln(stdout, "      --version           output version information and exit") //nolint:errcheck
}

// printVersion writes version information to stdout.
func printVersion(stdout *os.File) {
	fmt.Fprintf(stdout, "%s (go-unix-utils) %s\n", progName, version) //nolint:errcheck
}
