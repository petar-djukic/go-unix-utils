// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/dirname implements GNU dirname: strip last component from file paths.
//
// Implements prd016-dirname R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R3.1, R3.2, R3.3.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: dirname [OPTION] NAME...
Output each NAME with its last non-slash component and trailing slashes
removed; if NAME contains no /'s, output '.' (meaning the current directory).

  -z, --zero     end each output line with NUL, not newline
      --help     display this help and exit
      --version  output version information and exit

Examples:
  dirname /usr/bin/          -> "/usr"
  dirname dir1/str1 dir2/str2 -> "dir1" followed by "dir2"
`

const versionText = "dirname (go-unix-utils) 0.1\n"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and executes dirname logic.
func run(args []string, stdout, stderr *os.File) int {
	zero := false
	var names []string

	for i := 0; i < len(args); i++ {
		switch arg := args[i]; arg {
		case "--help":
			fmt.Fprint(stdout, helpText) //nolint:errcheck // best-effort
			return 0
		case "--version":
			fmt.Fprint(stdout, versionText) //nolint:errcheck // best-effort
			return 0
		case "--zero", "-z":
			zero = true
		case "--":
			names = append(names, args[i+1:]...)
			i = len(args)
		default:
			if parseShort(arg, &zero) {
				continue
			}
			names = append(names, args[i:]...)
			i = len(args)
		}
	}

	if len(names) == 0 {
		printMissingOperand(stderr)
		return 1
	}

	return printResults(names, zero, stdout)
}

// parseShort handles combined short flags like -zz.
func parseShort(arg string, zero *bool) bool {
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	for j := 1; j < len(arg); j++ {
		if arg[j] != 'z' {
			return false
		}
	}
	*zero = true
	return true
}

// printMissingOperand writes the missing-operand error to stderr.
func printMissingOperand(stderr *os.File) {
	fmt.Fprintln(stderr, "dirname: missing operand")                   //nolint:errcheck
	fmt.Fprintln(stderr, "Try 'dirname --help' for more information.") //nolint:errcheck
}

// printResults outputs dirname results for all names.
// R3.3: returns 1 on write error.
func printResults(names []string, zero bool, stdout *os.File) int {
	delim := "\n"
	if zero {
		delim = "\x00"
	}

	for _, name := range names {
		result := dirname(name)
		if _, err := fmt.Fprint(stdout, result+delim); err != nil {
			return 1
		}
	}
	return 0
}

// dirname extracts the directory component from a path.
// R1.1: strip trailing slashes, remove last component.
// R1.2: if no slash, return ".".
// R1.3: if all slashes, return "/".
// R1.4: strip trailing slashes from result.
func dirname(name string) string {
	if name == "" {
		return "."
	}

	// R1.3: all slashes → "/"
	if allSlashes(name) {
		return "/"
	}

	// R1.1: strip trailing slashes
	name = strings.TrimRight(name, "/")

	// R1.2: no slash → "."
	idx := strings.LastIndex(name, "/")
	if idx < 0 {
		return "."
	}

	// Remove last component
	dir := name[:idx]
	if dir == "" {
		return "/"
	}

	// R1.4: strip trailing slashes from result
	dir = strings.TrimRight(dir, "/")
	if dir == "" {
		return "/"
	}
	return dir
}

// allSlashes returns true if s consists entirely of '/' characters.
func allSlashes(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '/' {
			return false
		}
	}
	return true
}
