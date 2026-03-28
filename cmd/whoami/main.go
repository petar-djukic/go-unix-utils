// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd042-whoami: Print Effective User Name.
// Covers R1.1-R1.2 (default behavior, effective UID resolution),
// R2.1-R2.2 (extra operand/unknown flag errors),
// R3.1-R3.2 (differential testing).
package main

import (
	"fmt"
	"os"
	"os/user"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

// run parses arguments and prints the effective username. Returns exit code.
func run(args []string) int {
	for _, arg := range args {
		switch arg {
		case "--help":
			return printHelp()
		case "--version":
			return printVersion()
		case "--":
			// Ignore, but any following args are extra operands.
		default:
			if len(arg) > 1 && arg[0] == '-' {
				// R2.2: unknown flags produce an error.
				fmt.Fprintf(os.Stderr, "whoami: unrecognized option '%s'\n", arg)
				fmt.Fprintln(os.Stderr, "Try 'whoami --help' for more information.")
				return 1
			}
			// R2.1: extra operand.
			fmt.Fprintf(os.Stderr, "whoami: extra operand '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'whoami --help' for more information.")
			return 1
		}
	}

	// R1.1, R1.2: resolve effective UID to username.
	name, err := effectiveUsername()
	if err != nil {
		fmt.Fprintf(os.Stderr, "whoami: %v\n", err)
		return 1
	}

	if _, err := fmt.Println(name); err != nil {
		return 1
	}
	return 0
}

// effectiveUsername returns the username for the effective UID.
func effectiveUsername() (string, error) {
	uid := os.Geteuid()
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return "", fmt.Errorf("cannot find name for user ID %d: %w", uid, err)
	}
	return u.Username, nil
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: whoami [OPTION]...
Print the user name associated with the current effective user ID.
Same as id -un.

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
	_, err := fmt.Fprintf(os.Stdout, "whoami (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
