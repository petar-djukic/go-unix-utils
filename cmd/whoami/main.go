// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd042-whoami R1.1 (print effective username),
// R1.2 (exit 0 on success), R2.1 (--help flag), R2.2 (--version flag).
package main

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is used in error messages.
const programName = "whoami"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and prints the effective username.
// Returns the exit code.
func run(args []string) int {
	if err := parseArgs(args); err != nil {
		printError(err.Error())
		return 1
	}
	u, err := user.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot find username for your UID\n",
			programName)
		return 1
	}
	// R1.1: print effective username followed by a newline.
	fmt.Println(u.Username)
	return 0
}

// parseArgs validates arguments. Handles --help and --version,
// rejects unknown flags and extra operands.
func parseArgs(args []string) error {
	if len(args) == 0 {
		return nil
	}
	arg := args[0]
	if arg == "--help" || arg == "--version" {
		fmt.Print(helpText(arg))
		os.Exit(0)
	}
	if strings.HasPrefix(arg, "-") {
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	return fmt.Errorf("extra operand '%s'", arg)
}

// helpText returns text for --help or --version.
func helpText(flag string) string {
	if flag == "--version" {
		return "whoami (go-unix-utils) 1.0\n"
	}
	return `Usage: whoami [OPTION]...
Print the user name associated with the current effective user ID.
Same as id -un.

      --help     display this help and exit
      --version  output version information and exit
`
}

// printError writes a formatted error message to stderr.
func printError(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\nTry '%s --help' for more information.\n",
		programName, msg, programName)
}
