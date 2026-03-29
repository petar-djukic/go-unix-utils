// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/hostname prints the system hostname.
// Implements prd047-hostname R1.1, R1.2, R2.1, R2.2.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error messages.
const programName = "hostname"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	exit, done := parseArgs(os.Args[1:])
	if done {
		os.Exit(exit)
	}

	// R1.1: print the system hostname followed by a newline.
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}
	// R1.2: exit 0 on success.
	fmt.Println(hostname)
}

// parseArgs processes command-line arguments.
// Returns (exitCode, true) when the program should exit immediately.
func parseArgs(args []string) (int, bool) {
	if len(args) == 0 {
		return 0, false
	}
	arg := args[0]
	// R2.1: extra operands produce an error.
	if !strings.HasPrefix(arg, "-") || arg == "-" {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", programName, arg)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		return 1, true
	}
	// R2.2: only --help and --version are accepted.
	if strings.HasPrefix(arg, "--") {
		return handleLongOption(arg)
	}
	// Any short flag is unknown.
	fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, arg[1])
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
	return 1, true
}

// handleLongOption handles --help, --version and rejects unknown long options.
func handleLongOption(arg string) (int, bool) {
	// R2.2: --help prints usage information to stdout and exits 0.
	if arg == "--help" {
		printHelp()
		return 0, true
	}
	// R2.1: --version prints version information and exits 0.
	if arg == "--version" {
		fmt.Printf("%s (go-unix-utils) %s\n", programName, version)
		return 0, true
	}
	fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
	return 1, true
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Printf("Usage: %s [OPTION]\n", programName)
	fmt.Println("Print the system hostname.")
	fmt.Println()
	fmt.Println("      --help     display this help and exit")
	fmt.Println("      --version  output version information and exit")
}
