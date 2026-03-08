// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the rmdir utility for removing empty directories.
//
// Implements prd035-rmdir: basic empty directory removal (R1), parent directory
// removal (R2), error handling and verbosity (R3), differential testing (R4).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// flags holds the parsed command-line options.
type flags struct {
	parents             bool // -p, --parents: remove directory and its ancestors
	verbose             bool // -v, --verbose: print a message for each removed directory
	ignoreFailNonEmpty  bool // --ignore-fail-on-non-empty: suppress non-empty errors
}

func main() {
	sys.InstallSIGPIPEHandler()

	f, dirs := parseArgs(os.Args[1:])

	if len(dirs) == 0 {
		fmt.Fprintf(os.Stderr, "rmdir: missing operand\nTry 'rmdir --help' for more information.\n")
		os.Exit(1)
	}

	exitCode := 0
	for _, dir := range dirs {
		if err := removeDir(dir, f); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into flags and directory names.
func parseArgs(args []string) (flags, []string) {
	var f flags
	var dirs []string
	endFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endFlags || len(arg) == 0 || arg[0] != '-' || arg == "-" {
			dirs = append(dirs, arg)
			continue
		}

		if arg == "--" {
			endFlags = true
			continue
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--parents":
				f.parents = true
			case "--verbose":
				f.verbose = true
			case "--ignore-fail-on-non-empty":
				f.ignoreFailNonEmpty = true
			default:
				fmt.Fprintf(os.Stderr, "rmdir: unrecognized option '%s'\n", arg)
				os.Exit(1)
			}
			continue
		}

		// Short options.
		for j := 1; j < len(arg); j++ {
			switch arg[j] {
			case 'p':
				f.parents = true
			case 'v':
				f.verbose = true
			default:
				fmt.Fprintf(os.Stderr, "rmdir: invalid option -- '%c'\n", arg[j])
				os.Exit(1)
			}
		}
	}

	return f, dirs
}

// removeDir removes a single directory, handling -p, --ignore-fail-on-non-empty,
// and -v flags. R1, R2, R3.
func removeDir(path string, f flags) error {
	if err := doRemove(path, f, false); err != nil {
		return err
	}

	// R2.1: with -p, remove each successive parent component.
	if f.parents {
		current := filepath.Clean(path)
		for {
			parent := filepath.Dir(current)
			if parent == current || parent == "." || parent == "/" {
				break
			}
			if err := doRemove(parent, f, true); err != nil {
				return err
			}
			current = parent
		}
	}

	return nil
}

// doRemove attempts to remove a single directory and handles error suppression
// and verbose output. When isParent is true, the error message uses GNU's
// "failed to remove directory" format (used during -p ancestor removal).
func doRemove(path string, f flags, isParent bool) error {
	err := syscall.Rmdir(path)
	if err != nil {
		if f.ignoreFailNonEmpty && isDirNotEmpty(err) {
			return nil
		}
		return formatRmdirError(path, err, isParent)
	}

	// R3.3: verbose output for each successfully removed directory.
	if f.verbose {
		fmt.Printf("rmdir: removing directory, '%s'\n", path)
	}

	return nil
}

// isDirNotEmpty returns true if the error indicates the directory is not empty.
// R3.1, R3.2: only suppress ENOTEMPTY and EEXIST, not other errors.
func isDirNotEmpty(err error) bool {
	return err == syscall.ENOTEMPTY || err == syscall.EEXIST
}

// formatRmdirError formats a syscall error into GNU rmdir's error format.
// GNU rmdir uses "failed to remove directory" during -p ancestor removal
// and "failed to remove" for direct arguments.
func formatRmdirError(path string, err error, isParent bool) error {
	reason := err.Error()
	if errno, ok := err.(syscall.Errno); ok {
		reason = capitalizeFirst(errno.Error())
	}
	verb := "failed to remove"
	if isParent {
		verb = "failed to remove directory"
	}
	return fmt.Errorf("rmdir: %s '%s': %s", verb, path, reason)
}

// capitalizeFirst returns s with the first byte uppercased.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
