// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/whoami: print the effective user name.
// Implements srd042-whoami R1.1, R1.2, R2.1, R2.2.
package main

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "whoami"

// versionText is printed when --version is passed.
const versionText = progName + " (go-unix-utils)"

// helpText is the usage message printed when --help is passed.
const helpText = `Usage: whoami [OPTION]...
Print the user name associated with the current effective user ID.
Same as id -un.

      --help     display this help and exit
      --version  output version information and exit
`

func main() {
	sys.InstallSIGPIPEHandler()

	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}
}

// run processes arguments and prints the effective username.
// R1.1: no arguments prints effective username with trailing newline.
// R2.1, R2.2: extra operands and unknown flags produce errors.
func run(args []string) error {
	if err := parseArgs(args); err != nil {
		return err
	}

	// R1.1, R1.2: print the effective username via os/user.Current().
	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("cannot find name for user ID %d", os.Geteuid())
	}
	fmt.Println(u.Username)
	return nil
}

// parseArgs validates that no extra operands or unknown flags are present.
// R2.2: only --help and --version are accepted.
func parseArgs(args []string) error {
	if len(args) == 0 {
		return nil
	}
	arg := args[0]
	if arg == "--help" {
		fmt.Print(helpText)
		os.Exit(0)
	}
	if arg == "--version" {
		fmt.Println(versionText)
		os.Exit(0)
	}
	if strings.HasPrefix(arg, "-") && arg != "-" {
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	return fmt.Errorf("extra operand '%s'", arg)
}
