// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/version, a binary that prints the repository's
// last known version tag.
//
// Implements prd059-version R1.1–R1.5.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// R1.2: defaults to "dev" when the linker variable is not set.
var version = "dev"

// Version returns the version string for use by other cmd/ packages.
// R1.5: exported function so other cmd/ packages can import and report
// the same version string without duplicating the ldflags mechanism.
func Version() string {
	return version
}

// main is the entry point for cmd/version.
// R1.1: prints the version string to stdout followed by a newline and exits 0.
// R1.4: accepts --version/-v flags; rejects unknown flags with usage to stderr.
func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

// run parses arguments and dispatches to the appropriate action.
// Returns the exit code.
func run(args []string) int {
	if len(args) == 0 {
		printVersion()
		return 0
	}
	return handleFlags(args)
}

// handleFlags processes command-line flags.
// R1.4: --version and -v print the version string; any other flag
// prints a usage message to stderr and exits 2.
func handleFlags(args []string) int {
	switch args[0] {
	case "--version", "-v":
		printVersion()
		return 0
	default:
		printUsage()
		return 2
	}
}

// printVersion writes the version string to stdout.
// R1.1: outputs version followed by a newline.
func printVersion() {
	fmt.Println(version)
}

// printUsage writes a usage message to stderr.
// R1.4: displayed when an unrecognized flag is provided.
func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: version [--version | -v]")
}
