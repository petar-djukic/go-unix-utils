// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd058-rm R1.1–R1.4: basic file removal and argument handling.
// Implements prd058-rm R2.1–R2.4: recursive removal, force mode, and -d flag.
// Implements prd058-rm R3.3: -v/--verbose flag.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "rm"

// rmConfig holds parsed flag state.
type rmConfig struct {
	force     bool // R2.2: ignore nonexistent, never prompt
	recursive bool // R2.1: remove directories recursively
	dir       bool // R2.4: remove empty directories
	verbose   bool // R3.3: print each removal
}

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and executes the removal, returning the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	cfg, files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if len(files) == 0 {
		if cfg.force {
			return 0
		}
		fmt.Fprintf(stderr, "%s: missing operand\n", progName)
		printTryHelp(stderr)
		return 1
	}
	return removeAll(files, cfg, stdout, stderr)
}

// removeAll removes each file, returning 0 on full success, 1 on any error.
// R1.4: continues with remaining files on error.
func removeAll(files []string, cfg rmConfig, stdout, stderr io.Writer) int {
	exitCode := 0
	for _, f := range files {
		if err := removeSingle(f, cfg, stdout, stderr); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

// removeSingle removes one file or directory. R1.1, R1.2, R1.3, R2.1, R2.4.
func removeSingle(path string, cfg rmConfig, stdout, stderr io.Writer) error {
	// R1.3: refuse to remove "." or ".."
	base := filepath.Base(path)
	if base == "." || base == ".." {
		fmt.Fprintf(stderr,
			"%s: refusing to remove '.' or '..' directory: skipping '%s'\n",
			progName, path)
		return fmt.Errorf("refused")
	}
	info, err := os.Lstat(path)
	if err != nil {
		if cfg.force && os.IsNotExist(err) {
			return nil // R2.2: silently ignore nonexistent with -f
		}
		fmt.Fprintf(stderr, "%s: cannot remove '%s': %s\n",
			progName, path, unwrapPathError(err))
		return err
	}
	if info.IsDir() {
		return removeDirectory(path, cfg, stdout, stderr)
	}
	return removeFile(path, cfg, stdout, stderr)
}

// removeDirectory handles directory removal based on flags.
// R2.1: -r removes recursively. R2.4: -d removes empty directories.
// R1.2: without -r or -d, directory removal fails.
func removeDirectory(path string, cfg rmConfig, stdout, stderr io.Writer) error {
	if cfg.recursive {
		return removeRecursive(path, cfg, stdout, stderr)
	}
	if cfg.dir {
		return removeEmptyDir(path, cfg, stdout, stderr)
	}
	fmt.Fprintf(stderr, "%s: cannot remove '%s': Is a directory\n",
		progName, path)
	return fmt.Errorf("is a directory")
}

// removeRecursive removes a directory tree depth-first. R2.1, R2.3.
func removeRecursive(path string, cfg rmConfig, stdout, stderr io.Writer) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot remove '%s': %s\n",
			progName, path, unwrapPathError(err))
		return err
	}
	var firstErr error
	for _, entry := range entries {
		child := filepath.Join(path, entry.Name())
		if err := removeSingle(child, cfg, stdout, stderr); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if firstErr != nil {
		return firstErr
	}
	return removeEmptyDir(path, cfg, stdout, stderr)
}

// removeEmptyDir removes a single empty directory. R2.4.
func removeEmptyDir(path string, cfg rmConfig, stdout, stderr io.Writer) error {
	if err := os.Remove(path); err != nil {
		fmt.Fprintf(stderr, "%s: cannot remove '%s': %s\n",
			progName, path, unwrapPathError(err))
		return err
	}
	if cfg.verbose {
		fmt.Fprintf(stdout, "removed directory '%s'\n", path)
	}
	return nil
}

// removeFile removes a regular file (or symlink). R1.1.
func removeFile(path string, cfg rmConfig, stdout, stderr io.Writer) error {
	if err := os.Remove(path); err != nil {
		fmt.Fprintf(stderr, "%s: cannot remove '%s': %s\n",
			progName, path, unwrapPathError(err))
		return err
	}
	if cfg.verbose {
		// R3.3: GNU rm prints verbose to stdout
		fmt.Fprintf(stdout, "removed '%s'\n", path)
	}
	return nil
}

// parseArgs separates flags from file arguments and builds config.
// Returns config, file list, and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (rmConfig, []string, int) {
	var cfg rmConfig
	var files []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if len(arg) > 2 && arg[1] == '-' {
			code := applyLongFlag(arg, &cfg, stdout, stderr)
			if code >= 0 {
				return cfg, nil, code
			}
			continue
		}
		code := applyShortFlags(arg, &cfg, stderr)
		if code >= 0 {
			return cfg, nil, code
		}
	}
	return cfg, files, -1
}

// applyShortFlags processes combined short flags.
func applyShortFlags(arg string, cfg *rmConfig, stderr io.Writer) int {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'f':
			cfg.force = true
		case 'r', 'R':
			cfg.recursive = true // R2.1
		case 'd':
			cfg.dir = true // R2.4
		case 'v':
			cfg.verbose = true
		default:
			fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n",
				progName, arg[j])
			printTryHelp(stderr)
			return 1
		}
	}
	return -1
}

// applyLongFlag handles --long-name flags.
func applyLongFlag(arg string, cfg *rmConfig, stdout, stderr io.Writer) int {
	switch arg {
	case "--help":
		printHelp(stdout)
		return 0
	case "--version":
		printVersion(stdout)
		return 0
	case "--force":
		cfg.force = true
		return -1
	case "--recursive":
		cfg.recursive = true // R2.1
		return -1
	case "--dir":
		cfg.dir = true // R2.4
		return -1
	case "--verbose":
		cfg.verbose = true
		return -1
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	}
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [FILE]...\n", progName)
	fmt.Fprintln(w, "Remove (unlink) the FILE(s).")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -d, --dir             remove empty directories")
	fmt.Fprintln(w, "  -f, --force           ignore nonexistent files and arguments, never prompt")
	fmt.Fprintln(w, "  -r, -R, --recursive   remove directories and their contents recursively")
	fmt.Fprintln(w, "  -v, --verbose         explain what is being done")
	fmt.Fprintln(w, "      --help            display this help and exit")
	fmt.Fprintln(w, "      --version         output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages (e.g., "No such file or directory").
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
