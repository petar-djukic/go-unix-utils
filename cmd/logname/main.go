// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd053-logname: Print Login Name.
// Covers R1.1-R1.2 (default behavior, login name from LOGNAME/os.user),
// R2.1-R2.3 (extra operand/unknown flag/lookup failure errors),
// R3.1 (SIGPIPE handling).
package main

import (
	"fmt"
	"os"
	"os/user"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

// run parses arguments and prints the login name. Returns exit code.
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
				fmt.Fprintf(os.Stderr, "logname: unrecognized option '%s'\n", arg)
				fmt.Fprintln(os.Stderr, "Try 'logname --help' for more information.")
				return 1
			}
			// R2.1: extra operand.
			fmt.Fprintf(os.Stderr, "logname: extra operand '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'logname --help' for more information.")
			return 1
		}
	}

	// R1.1, R1.2: print login name from LOGNAME env var or os/user fallback.
	name, err := loginName()
	if err != nil {
		// R2.3: login name cannot be determined.
		fmt.Fprintf(os.Stderr, "logname: %v\n", err)
		return 1
	}

	if _, err := fmt.Println(name); err != nil {
		return 1
	}
	return 0
}

// loginName returns the login name from LOGNAME or os/user.Current().
func loginName() (string, error) {
	if name := os.Getenv("LOGNAME"); name != "" {
		return name, nil
	}
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("no login name: %w", err)
	}
	return u.Username, nil
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: logname [OPTION]
Print the name of the current user.

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
	_, err := fmt.Fprintf(os.Stdout, "logname (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
