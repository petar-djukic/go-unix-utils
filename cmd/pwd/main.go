// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd051-pwd: Print Working Directory.
// Covers R1.1-R1.4 (default/logical/physical modes, last-flag-wins),
// R2.1-R2.2 (extra operand/unknown flag errors),
// R3.1-R3.3 (differential testing, coverage, LC_ALL=C).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// mode represents the working directory resolution mode.
type mode int

const (
	modePhysical mode = iota
	modeLogical
)

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

// run parses arguments and prints the working directory. Returns exit code.
func run(args []string) int {
	m, exitCode := parseArgs(args)
	if exitCode >= 0 {
		return exitCode
	}

	dir, err := resolveDir(m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pwd: %v\n", err)
		return 1
	}

	if _, err := fmt.Println(dir); err != nil {
		return 1
	}
	return 0
}

// parseArgs processes command-line arguments. Returns the resolved mode and
// an exit code. An exit code of -1 means parsing succeeded and execution
// should continue.
func parseArgs(args []string) (mode, int) {
	m := modePhysical // R1.4: default is -P.
	endOfOpts := false
	for _, arg := range args {
		if endOfOpts {
			// R2.1: GNU pwd ignores non-option arguments.
			continue
		}
		switch arg {
		case "--help":
			return m, printHelp()
		case "--version":
			return m, printVersion()
		case "-L", "--logical":
			m = modeLogical // R1.2, R1.4: last flag wins.
		case "-P", "--physical":
			m = modePhysical // R1.3, R1.4: last flag wins.
		case "--":
			endOfOpts = true
		default:
			if len(arg) > 1 && arg[0] == '-' {
				return m, printUnknownFlag(arg)
			}
			// R2.1: GNU pwd ignores non-option arguments with a warning.
			fmt.Fprintf(os.Stderr, "pwd: ignoring non-option arguments\n")
		}
	}
	return m, -1
}

// resolveDir returns the working directory path for the given mode.
func resolveDir(m mode) (string, error) {
	if m == modeLogical {
		if dir := logicalDir(); dir != "" {
			return dir, nil
		}
	}
	// R1.1, R1.3: physical path via os.Getwd (resolves symlinks).
	return os.Getwd()
}

// logicalDir returns the PWD environment variable value if it is a valid
// logical path for the current directory. Returns "" if PWD is unset,
// contains . or .. components, or does not name the current directory.
func logicalDir() string {
	pwd := os.Getenv("PWD")
	if pwd == "" || !strings.HasPrefix(pwd, "/") {
		return ""
	}
	if containsDotComponents(pwd) {
		return ""
	}
	return validatePWD(pwd)
}

// containsDotComponents reports whether path contains . or .. components.
func containsDotComponents(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == "." || part == ".." {
			return true
		}
	}
	return false
}

// validatePWD checks that pwd names the same directory as the physical cwd.
// Returns pwd if valid, "" otherwise.
func validatePWD(pwd string) string {
	pwdInfo, err := os.Stat(pwd)
	if err != nil {
		return ""
	}
	cwdInfo, err := os.Stat(".")
	if err != nil {
		return ""
	}
	if !os.SameFile(pwdInfo, cwdInfo) {
		return ""
	}
	return pwd
}

// printUnknownFlag writes an error for an unrecognized option and returns 1.
// R2.2, D1: format matches GNU pwd error messages.
func printUnknownFlag(flag string) int {
	if len(flag) == 2 && flag[0] == '-' {
		// Short flag: GNU uses "invalid option -- 'x'" format.
		fmt.Fprintf(os.Stderr, "pwd: invalid option -- '%c'\n", flag[1])
	} else {
		fmt.Fprintf(os.Stderr, "pwd: unrecognized option '%s'\n", flag)
	}
	fmt.Fprintln(os.Stderr, "Try 'pwd --help' for more information.")
	return 1
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: pwd [OPTION]...
Print the full filename of the current working directory.

  -L, --logical   use PWD from environment, even if it contains symlinks
  -P, --physical  avoid all symlinks
      --help     display this help and exit
      --version  output version information and exit

If no option is specified, -P is assumed.
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "pwd (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
