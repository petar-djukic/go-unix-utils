// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd084-link: Create a Hard Link.
// Covers R1.1-R1.4 (hard link creation, argument validation, error handling),
// R2.1-R2.3 (exit codes, SIGPIPE handling).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const progName = "link"

func main() {
	// R2.3: handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run validates arguments and creates the hard link. Returns exit code.
func run(args []string) int {
	if err := handleSpecialFlags(args); err != nil {
		return 0
	}
	if code := validateArgs(args); code >= 0 {
		return code
	}
	return createLink(args[0], args[1])
}

// handleSpecialFlags checks for --help and --version.
// Returns a non-nil error (as sentinel) when a flag was handled.
func handleSpecialFlags(args []string) error {
	for _, arg := range args {
		switch arg {
		case "--help":
			printHelp()
			return fmt.Errorf("handled")
		case "--version":
			printVersion()
			return fmt.Errorf("handled")
		}
	}
	return nil
}

// validateArgs checks that exactly two arguments are provided.
// R1.3: exits 1 with error to stderr if fewer or more than two.
// Returns -1 to proceed, >= 0 as exit code.
func validateArgs(args []string) int {
	switch len(args) {
	case 0:
		fmt.Fprintf(os.Stderr,
			"%s: missing operand\n", progName)
		printTryHelp()
		return 1
	case 1:
		fmt.Fprintf(os.Stderr,
			"%s: missing operand after '%s'\n", progName, args[0])
		printTryHelp()
		return 1
	case 2:
		return -1
	default:
		fmt.Fprintf(os.Stderr,
			"%s: extra operand '%s'\n", progName, args[2])
		printTryHelp()
		return 1
	}
}

// printTryHelp prints the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr,
		"Try '%s --help' for more information.\n", progName)
}

// createLink creates a hard link from source to dest using os.Link.
// R1.1: uses link(2) semantics via os.Link.
// R1.2: does not follow symlinks or handle directories specially.
// R1.4: exits 1 with error to stderr if link(2) fails.
func createLink(source, dest string) int {
	if err := os.Link(source, dest); err != nil {
		printLinkError(source, dest, err)
		return 1
	}
	return 0
}

// printLinkError formats a link error matching GNU glink format.
// R1.3: format is "link: cannot create link 'FILE2' to 'FILE1': Reason".
func printLinkError(source, dest string, err error) {
	reason := err.Error()
	if le, ok := err.(*os.LinkError); ok {
		reason = le.Err.Error()
	}
	fmt.Fprintf(os.Stderr,
		"%s: cannot create link '%s' to '%s': %s\n",
		progName, dest, source, capitalizeFirst(reason))
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Fprint(os.Stdout, `Usage: link FILE1 FILE2
  or:  link OPTION
Call the link function to create a link named FILE2 to an existing FILE1.

      --help     display this help and exit
      --version  output version information and exit
`)
}

// printVersion writes version information to stdout.
func printVersion() {
	fmt.Fprintf(os.Stdout,
		"link (go-unix-utils) %s\n", version)
}
