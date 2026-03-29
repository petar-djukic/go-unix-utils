// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tty implements GNU tty: print the terminal file name connected to stdin.
//
// Implements prd052-tty R1.1, R1.2, R1.3, R2.1, R2.2.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "tty"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and prints the terminal name. Returns exit code.
// R1.1: exit 0 when stdin is a terminal.
// R1.2: exit 1 when stdin is not a terminal.
// R1.3: -s suppresses output; exit code still indicates terminal status.
func run(args []string, stdout, stderr *os.File) int {
	silent, result, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err) //nolint:errcheck
		printTryHelp(stderr)
		return 2
	}
	if result == resultHelp {
		printHelp(stdout)
		return 0
	}
	if result == resultVersion {
		printVersion(stdout)
		return 0
	}
	return printTTY(silent, stdout)
}

// printTTY detects whether stdin is a terminal and prints the device name.
// R1.1: prints device path and exits 0 when stdin is a terminal.
// R1.2: prints "not a tty" and exits 1 when stdin is not a terminal.
// R1.3: -s suppresses all output.
func printTTY(silent bool, stdout *os.File) int {
	isTerm := sys.IsTerminal(os.Stdin.Fd())
	if !silent {
		if isTerm {
			name := ttyName()
			fmt.Fprintln(stdout, name) //nolint:errcheck
		} else {
			fmt.Fprintln(stdout, "not a tty") //nolint:errcheck
		}
	}
	if isTerm {
		return 0
	}
	return 1
}

// ttyName returns the terminal device path for stdin.
func ttyName() string {
	// TODO: R1.1 — implementation task will resolve the actual device path.
	return "/dev/tty"
}

// parseResult describes the outcome of argument parsing.
type parseResult int

const (
	resultContinue parseResult = iota
	resultHelp
	resultVersion
)

// parseArgs extracts the silent flag from command-line arguments.
// R1.3: -s, --silent, --quiet enable silent mode.
// R2.1: extra operands produce an error and exit 2.
// R2.2: unknown flags produce an error and exit 2.
func parseArgs(args []string) (bool, parseResult, error) {
	silent := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return silent, resultContinue, fmt.Errorf("extra operand '%s'", arg)
		}
		s, result, err := parseFlag(arg, silent)
		if err != nil {
			return silent, resultContinue, err
		}
		if result != resultContinue {
			return s, result, nil
		}
		silent = s
	}
	return silent, resultContinue, nil
}

// parseFlag parses a single flag argument and returns the updated silent state.
func parseFlag(arg string, current bool) (bool, parseResult, error) {
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(arg, current)
	}
	s, err := parseShortFlags(arg, current)
	return s, resultContinue, err
}

// parseLongFlag handles --silent, --quiet, --help, and --version.
func parseLongFlag(arg string, current bool) (bool, parseResult, error) {
	switch arg {
	case "--silent", "--quiet":
		return true, resultContinue, nil
	case "--help":
		return current, resultHelp, nil
	case "--version":
		return current, resultVersion, nil
	default:
		return current, resultContinue, fmt.Errorf("unrecognized option '%s'", arg)
	}
}

// parseShortFlags handles short flag bundles like -s.
func parseShortFlags(arg string, current bool) (bool, error) {
	s := current
	for _, ch := range arg[1:] {
		switch ch {
		case 's':
			s = true
		default:
			return s, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return s, nil
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(stderr *os.File) {
	fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
}

// printHelp writes usage information to stdout.
func printHelp(stdout *os.File) {
	fmt.Fprintf(stdout, "Usage: %s [OPTION]...\n", progName)                          //nolint:errcheck
	fmt.Fprintln(stdout, "Print the file name of the terminal connected to standard input.") //nolint:errcheck
	fmt.Fprintln(stdout)                                                               //nolint:errcheck
	fmt.Fprintln(stdout, "  -s, --silent, --quiet   print nothing, only return an exit status") //nolint:errcheck
	fmt.Fprintln(stdout, "      --help              display this help and exit")       //nolint:errcheck
	fmt.Fprintln(stdout, "      --version           output version information and exit") //nolint:errcheck
}

// printVersion writes version information to stdout.
func printVersion(stdout *os.File) {
	fmt.Fprintf(stdout, "%s (go-unix-utils) %s\n", progName, version) //nolint:errcheck
}
