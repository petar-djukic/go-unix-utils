// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd035-rmdir R1.1–R1.4, R2.1–R2.3, R3.1: rmdir core,
// parents mode, and ignore-fail-on-non-empty.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "rmdir"

// options holds the parsed command-line flags for rmdir.
type options struct {
	parents        bool // R2.1: -p, --parents
	ignoreNonEmpty bool // R3.1: --ignore-fail-on-non-empty
}

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and removes directories, returning the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	dirs, opts, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if len(dirs) == 0 {
		fmt.Fprintf(stderr, "%s: missing operand\n", progName)
		printTryHelp(stderr)
		return 1
	}
	return removeDirs(dirs, opts, stderr)
}

// parseArgs separates flags from directory arguments.
// Returns directory list, options, and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) ([]string, options, int) {
	var dirs []string
	var opts options
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
			code := applyLongFlag(arg, &opts, stdout, stderr)
			if code >= 0 {
				return nil, options{}, code
			}
			continue
		}
		code := applyShortFlags(arg, &opts, stderr)
		if code >= 0 {
			return nil, options{}, code
		}
	}
	return dirs, opts, -1
}

// applyShortFlags processes combined short flags (e.g., -p).
// Returns exit code >= 0 for terminal flags, -1 to continue.
func applyShortFlags(arg string, opts *options, stderr io.Writer) int {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'p':
			// R2.1: enable parent removal mode.
			opts.parents = true
		default:
			fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", progName, arg[j])
			printTryHelp(stderr)
			return 1
		}
	}
	return -1
}

// applyLongFlag handles --long-name flags.
// Returns exit code >= 0 for terminal flags, -1 to continue.
func applyLongFlag(arg string, opts *options, stdout, stderr io.Writer) int {
	switch arg {
	case "--help":
		printHelp(stdout)
		return 0
	case "--version":
		printVersion(stdout)
		return 0
	case "--parents":
		opts.parents = true
	case "--ignore-fail-on-non-empty":
		opts.ignoreNonEmpty = true
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	}
	return -1
}

// removeDirs removes each directory independently, returning 0 or 1.
// R1.2, R2.3: processes each directory argument independently.
func removeDirs(dirs []string, opts options, stderr io.Writer) int {
	exitCode := 0
	for _, dir := range dirs {
		if code := removeOne(dir, opts, stderr); code != 0 {
			exitCode = code
		}
	}
	return exitCode
}

// removeOne removes a single directory, with optional parent removal.
// R1.1: removes the target. R2.1: ascends to parents when -p is set.
func removeOne(dir string, opts options, stderr io.Writer) int {
	if err := os.Remove(dir); err != nil {
		if shouldIgnore(err, opts) {
			return 0
		}
		fmt.Fprintf(stderr, "%s: failed to remove '%s': %s\n",
			progName, dir, unwrapPathError(err))
		return 1
	}
	if opts.parents {
		return removeParents(dir, opts, stderr)
	}
	return 0
}

// removeParents removes successive empty parent directories after the
// target has been removed. R2.1: ascends the path removing each parent.
// R2.2: stops when a parent cannot be removed.
func removeParents(dir string, opts options, stderr io.Writer) int {
	current := filepath.Clean(dir)
	for {
		parent := filepath.Dir(current)
		if parent == current || parent == "." {
			break
		}
		if err := os.Remove(parent); err != nil {
			if shouldIgnore(err, opts) {
				return 0
			}
			fmt.Fprintf(stderr, "%s: failed to remove directory '%s': %s\n",
				progName, parent, unwrapPathError(err))
			return 1
		}
		current = parent
	}
	return 0
}

// shouldIgnore returns true when the error should be suppressed by
// --ignore-fail-on-non-empty. R3.1: suppresses only non-empty errors.
func shouldIgnore(err error, opts options) bool {
	if !opts.ignoreNonEmpty {
		return false
	}
	return isNotEmptyError(err)
}

// isNotEmptyError reports whether err indicates a non-empty directory.
func isNotEmptyError(err error) bool {
	return errors.Is(err, syscall.ENOTEMPTY)
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
	fmt.Fprintln(w, "      --ignore-fail-on-non-empty")
	fmt.Fprintln(w, "                    ignore each failure that is solely because a directory")
	fmt.Fprintln(w, "                      is non-empty")
	fmt.Fprintln(w, "  -p, --parents     remove DIRECTORY and its ancestors; e.g., 'rmdir -p a/b/c' is")
	fmt.Fprintln(w, "                      similar to 'rmdir a/b/c a/b a'")
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
