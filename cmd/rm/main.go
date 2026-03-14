// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd058-rm R1.1-R1.4, R2.2, R3.3
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error messages.
const programName = "rm"

// rmOpts holds all flag state for an rm invocation.
type rmOpts struct {
	force   bool // -f: ignore nonexistent files, never prompt
	verbose bool // -v: print each removal
}

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var operands []string
	var opts rmOpts

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--force":
			opts.force = true
		case arg == "--verbose":
			opts.verbose = true
		case arg == "--version":
			fmt.Println("rm (go-unix-utils) 0.1")
			os.Exit(0)
		case arg == "--help":
			printUsage()
			os.Exit(0)
		case arg == "--":
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "--"):
			// Unrecognized long option.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Parse bundled short options.
			flags := arg[1:]
			for j := 0; j < len(flags); j++ {
				switch flags[j] {
				case 'f':
					opts.force = true
				case 'v':
					opts.verbose = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, flags[j])
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
					os.Exit(1)
				}
			}
		default:
			operands = append(operands, arg)
		}
	}

	// R1.1: With no operands, print usage to stderr and exit 1.
	// R2.2: With -f and no operands, GNU rm exits 0 silently.
	if len(operands) == 0 {
		if opts.force {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	exitCode := 0
	for _, path := range operands {
		if err := removeFile(path, opts); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
			exitCode = 1
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// removeFile removes a single file at path, respecting the options.
//
// R1.1: Remove files using os.Remove (unlink).
// R1.2: Without -r, refuse to remove directories.
// R1.3: Refuse to remove '.' or '..'.
// R1.4: Print error and continue on failure.
// R2.2: -f suppresses errors for nonexistent files.
// R3.3: -v prints "removed '<path>'" to stdout.
func removeFile(path string, opts rmOpts) error {
	// Check if the path exists.
	info, statErr := os.Lstat(path)
	if statErr != nil {
		// R2.2: -f suppresses errors for nonexistent files.
		if opts.force && errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(statErr))
	}

	// R1.2: without -r, refuse to remove directories.
	// R1.3: '.' and '..' are directories, so this check covers them too.
	// The special "refusing to remove '.' or '..'" message is emitted only
	// with -r (implemented in a later task). Without -r, the generic
	// directory error matches GNU rm behavior.
	if info.IsDir() {
		return fmt.Errorf("cannot remove '%s': Is a directory", path)
	}

	// R1.1: remove the file.
	if err := os.Remove(path); err != nil {
		// R2.2: -f suppresses errors for nonexistent files (race condition).
		if opts.force && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(err))
	}

	// R3.3: -v prints each removal to stdout.
	if opts.verbose {
		fmt.Fprintf(os.Stdout, "removed '%s'\n", path)
	}

	return nil
}

// printUsage prints a brief usage message to stdout.
func printUsage() {
	fmt.Println("Usage: rm [OPTION]... [FILE]...")
	fmt.Println("Remove (unlink) the FILE(s).")
	fmt.Println()
	fmt.Println("  -f, --force           ignore nonexistent files and arguments, never prompt")
	fmt.Println("  -v, --verbose         explain what is being done")
	fmt.Println("      --help            display this help and exit")
	fmt.Println("      --version         output version information and exit")
}

// sysErrMsg extracts the underlying syscall error message string from a
// (possibly wrapped) error, producing GNU-compatible messages like
// "No such file or directory" rather than Go's "stat /path: no such file...".
func sysErrMsg(err error) string {
	var msg string
	var errno syscall.Errno
	if errors.As(err, &errno) {
		msg = errno.Error()
	} else {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			msg = pathErr.Err.Error()
		} else {
			msg = err.Error()
		}
	}
	return capitalizeFirst(msg)
}

// capitalizeFirst returns s with its first rune uppercased, matching GNU
// coreutils error message capitalization (e.g., "No such file or directory").
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}
