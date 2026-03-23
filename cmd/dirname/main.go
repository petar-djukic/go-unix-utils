// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd016-dirname: Strip Last Component from File Paths.
// Covers R1.1-R1.3 (core path stripping, no-slash paths, root paths),
// R2.1-R2.2 (help, version).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	names, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	if len(names) == 0 {
		fmt.Fprintln(os.Stderr, "dirname: missing operand")
		fmt.Fprintln(os.Stderr, "Try 'dirname --help' for more information.")
		os.Exit(1)
	}

	os.Exit(run(names))
}

// run processes each name and prints its directory portion. Returns exit code.
func run(names []string) int {
	for _, name := range names {
		if _, err := fmt.Println(dirname(name)); err != nil {
			return 1
		}
	}
	return 0
}

// dirname strips the last component from name, returning the directory portion.
// R1.1: strips trailing slashes, then removes the last component.
// R1.2: outputs "." when name contains no '/'.
// R1.3: outputs "/" when name is "/" or entirely slashes.
func dirname(name string) string {
	// R1.1: strip trailing slashes.
	trimmed := strings.TrimRight(name, "/")

	// R1.3: name was entirely slashes (or just "/").
	if trimmed == "" {
		return "/"
	}

	// R1.2: no slash means no directory component.
	i := strings.LastIndex(trimmed, "/")
	if i < 0 {
		return "."
	}

	// Remove the last component (everything after the final '/').
	result := trimmed[:i]

	// Strip trailing slashes from the result.
	result = strings.TrimRight(result, "/")

	// If empty after stripping, the directory is root.
	if result == "" {
		return "/"
	}

	return result
}

// parseArgs processes flags and returns positional arguments.
// exit is -1 when processing should continue; >= 0 for early termination.
func parseArgs(args []string) (names []string, exit int) {
	exit = -1
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			names = append(names, args[i+1:]...)
			return
		case arg == "--help":
			return nil, printHelp()
		case arg == "--version":
			return nil, printVersion()
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			fmt.Fprintf(os.Stderr, "dirname: unrecognized option '%s'\n", arg)
			return nil, 1
		default:
			names = append(names, args[i:]...)
			return
		}
	}
	return
}

// printHelp writes usage information to stdout and returns the exit code.
// R2.1: --help prints to stdout and exits 0.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: dirname [OPTION] NAME...
Output each NAME with its last non-slash component and trailing slashes
removed; if NAME contains no /'s, output '.' (meaning the current directory).

      --help     display this help and exit
      --version  output version information and exit

Examples:
  dirname /usr/bin/          -> "/usr"
  dirname dir1/str dir2/str  -> "dir1" followed by "dir2"
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
// R2.2: --version prints to stdout and exits 0.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "dirname (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
