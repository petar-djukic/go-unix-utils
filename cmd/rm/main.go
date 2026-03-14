// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd058-rm R1.1-R1.4, R2.1-R2.4, R3.3
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error messages.
const programName = "rm"

// errAlreadyReported signals that errors have been printed to stderr
// by the function that encountered them. The main loop should not
// print this error again.
var errAlreadyReported = errors.New("already reported")

// rmOpts holds all flag state for an rm invocation.
type rmOpts struct {
	force         bool // -f: ignore nonexistent files, never prompt
	recursive     bool // -r/-R: remove directories recursively
	dir           bool // -d: remove empty directories
	verbose       bool // -v: print each removal
	oneFileSystem bool // --one-file-system: skip directories on different devices
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
		case arg == "--recursive":
			opts.recursive = true
		case arg == "--dir":
			opts.dir = true
		case arg == "--verbose":
			opts.verbose = true
		case arg == "--one-file-system":
			opts.oneFileSystem = true
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
				case 'r', 'R':
					opts.recursive = true
				case 'd':
					opts.dir = true
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
			if !errors.Is(err, errAlreadyReported) {
				fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
			}
			exitCode = 1
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// removeFile removes a single file or directory at path, respecting the options.
//
// R1.1: Remove files using os.Remove (unlink).
// R1.2: Without -r or -d, refuse to remove directories.
// R1.3: Refuse to remove '.' or '..'.
// R1.4: Print error and continue on failure.
// R2.1: -r removes directories recursively.
// R2.2: -f suppresses errors for nonexistent files.
// R2.4: -d removes empty directories.
// R3.3: -v prints removal messages to stdout.
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

	if info.IsDir() {
		// R1.3/D3: Reject '.' and '..' when -r or -d is active.
		base := filepath.Base(path)
		if (opts.recursive || opts.dir) && (base == "." || base == "..") {
			return fmt.Errorf("refusing to remove '.' or '..' directory: skipping '%s'", path)
		}

		// R2.1: -r removes directories recursively.
		if opts.recursive {
			return removeRecursive(path, opts)
		}

		// R2.4: -d removes empty directories.
		if opts.dir {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(err))
			}
			if opts.verbose {
				fmt.Fprintf(os.Stdout, "removed directory '%s'\n", path)
			}
			return nil
		}

		// R1.2: Without -r or -d, refuse to remove directories.
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

// entry records a path and whether it is a directory, used for post-order removal.
type entry struct {
	path  string
	isDir bool
}

// removeRecursive removes a directory tree rooted at path. It collects entries
// via filepath.WalkDir and removes them in reverse order (children before
// parents) per D4.
//
// R2.1: Recursive directory removal.
// R2.3: --one-file-system skips directories on different devices.
// D2: Symlinks are removed but never followed.
// D4: Post-order removal via reversed WalkDir collection.
func removeRecursive(path string, opts rmOpts) error {
	var rootDev uint64
	if opts.oneFileSystem {
		fi, err := sys.Lstat(path)
		if err != nil {
			return fmt.Errorf("cannot remove '%s': %s", path, sysErrMsg(err))
		}
		rootDev = fi.Dev
	}

	var entries []entry
	hadError := false

	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Permission denied or other error accessing this entry.
			fmt.Fprintf(os.Stderr, "%s: cannot remove '%s': %s\n", programName, p, sysErrMsg(err))
			hadError = true
			return nil
		}

		// R2.3: --one-file-system skips directories on different devices.
		if opts.oneFileSystem && d.IsDir() && p != path {
			fi, statErr := sys.Lstat(p)
			if statErr != nil {
				fmt.Fprintf(os.Stderr, "%s: cannot remove '%s': %s\n", programName, p, sysErrMsg(statErr))
				hadError = true
				return filepath.SkipDir
			}
			if fi.Dev != rootDev {
				fmt.Fprintf(os.Stderr, "%s: skipping '%s', since it's on a different device\n", programName, p)
				return filepath.SkipDir
			}
		}

		entries = append(entries, entry{path: p, isDir: d.IsDir()})
		return nil
	})

	// D4: Remove in reverse order (children before parents).
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if err := os.Remove(e.path); err != nil {
			if opts.force && errors.Is(err, os.ErrNotExist) {
				continue
			}
			fmt.Fprintf(os.Stderr, "%s: cannot remove '%s': %s\n", programName, e.path, sysErrMsg(err))
			hadError = true
			continue
		}
		// R3.3: -v prints each removal to stdout.
		if opts.verbose {
			if e.isDir {
				fmt.Fprintf(os.Stdout, "removed directory '%s'\n", e.path)
			} else {
				fmt.Fprintf(os.Stdout, "removed '%s'\n", e.path)
			}
		}
	}

	if hadError {
		return errAlreadyReported
	}
	return nil
}

// printUsage prints a brief usage message to stdout.
func printUsage() {
	fmt.Println("Usage: rm [OPTION]... [FILE]...")
	fmt.Println("Remove (unlink) the FILE(s).")
	fmt.Println()
	fmt.Println("  -f, --force           ignore nonexistent files and arguments, never prompt")
	fmt.Println("  -r, -R, --recursive   remove directories and their contents recursively")
	fmt.Println("  -d, --dir             remove empty directories")
	fmt.Println("  -v, --verbose         explain what is being done")
	fmt.Println("      --one-file-system when removing a hierarchy recursively, skip any")
	fmt.Println("                          directory on a different file system")
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
