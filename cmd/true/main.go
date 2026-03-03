// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/true exits with status 0 unconditionally. It silently ignores all
// arguments except --help and --version as the first argument. Matches
// GNU coreutils true behavior under LC_ALL=C.
//
// Implements prd020-true R1-R5.
package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "true"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// No arguments: exit 0 silently (R1.1).
	if len(args) == 0 {
		os.Exit(0)
	}

	// Only the first argument is checked for --help and --version (R2.3).
	switch args[0] {
	case "--help":
		printHelp()
	case "--version":
		printVersion()
	}

	// All other arguments are silently ignored (R1.2, R3.1, R3.2).
	os.Exit(0)
}

// printHelp writes a usage message to stdout and exits 0 (R2.1).
// On write error, exits 1 with a diagnostic on stderr (R5.2, R5.3).
func printHelp() {
	out := bufio.NewWriter(os.Stdout)
	fmt.Fprintf(out, "Usage: %s [ignored command line arguments]\n", progName)
	fmt.Fprintf(out, "  or:  %s OPTION\n", progName)
	fmt.Fprintln(out, "Exit with a status code indicating success.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "      --help     display this help and exit")
	fmt.Fprintln(out, "      --version  output version information and exit")

	if err := out.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		os.Exit(1)
	}

	os.Exit(0)
}

// printVersion writes version information to stdout and exits 0 (R2.2).
// On write error, exits 1 with a diagnostic on stderr (R5.2, R5.3).
func printVersion() {
	out := bufio.NewWriter(os.Stdout)
	fmt.Fprintf(out, "%s (%s) %s\n", progName, "go-unix-utils", version)

	if err := out.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		os.Exit(1)
	}

	os.Exit(0)
}
