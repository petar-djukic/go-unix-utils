// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd035-rmdir R1.1–R1.4: basic empty directory removal and error handling.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "rmdir"

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and removes directories, returning the exit code.
// R1.1: removes a single empty directory.
// R1.2: processes multiple directory arguments independently.
func run(args []string, stdout, stderr io.Writer) int {
	dirs, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if len(dirs) == 0 {
		fmt.Fprintf(stderr, "%s: missing operand\n", progName)
		printTryHelp(stderr)
		return 1
	}
	return removeDirs(dirs, stderr)
}

// parseArgs separates flags from directory arguments.
// Returns directory list and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) ([]string, int) {
	var dirs []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || len(arg) == 0 || arg[0] != '-' {
			dirs = append(dirs, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if len(arg) > 2 && arg[1] == '-' {
			code := applyLongFlag(arg, stdout, stderr)
			if code >= 0 {
				return nil, code
			}
			continue
		}
		code := applyShortFlags(arg, stderr)
		if code >= 0 {
			return nil, code
		}
	}
	return dirs, -1
}

// applyShortFlags processes combined short flags.
// Returns exit code >= 0 for terminal flags, -1 to continue.
func applyShortFlags(arg string, stderr io.Writer) int {
	for j := 1; j < len(arg); j++ {
		fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", progName, arg[j])
		printTryHelp(stderr)
		return 1
	}
	return -1
}

// applyLongFlag handles --long-name flags.
// Returns exit code >= 0 for terminal flags, -1 to continue.
func applyLongFlag(arg string, stdout, stderr io.Writer) int {
	switch arg {
	case "--help":
		printHelp(stdout)
		return 0
	case "--version":
		printVersion(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	}
}

// removeDirs removes each directory independently, returning 0 or 1.
// R1.2: processes each directory argument independently.
// R1.3: prints error to stderr when a directory is not empty.
// R1.4: prints error to stderr when target does not exist or is not a directory.
func removeDirs(dirs []string, stderr io.Writer) int {
	exitCode := 0
	for _, dir := range dirs {
		if err := os.Remove(dir); err != nil {
			fmt.Fprintf(stderr, "%s: failed to remove '%s': %s\n",
				progName, dir, unwrapPathError(err))
			exitCode = 1
		}
	}
	return exitCode
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... DIRECTORY...\n", progName)
	fmt.Fprintln(w, "Remove the DIRECTORY(ies), if they are empty.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "      --help        display this help and exit")
	fmt.Fprintln(w, "      --version     output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages (e.g., "Directory not empty").
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
