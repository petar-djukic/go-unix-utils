// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd058-rm R1.1–R1.4: basic file removal and argument handling.
// Implements prd058-rm R2.2: -f/--force flag (ignore nonexistent, never prompt).
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
	force   bool // R2.2: ignore nonexistent, never prompt
	verbose bool // R3.3: print each removal
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

// removeSingle removes one file. R1.1, R1.2, R1.3.
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
	// R1.2: without -r, refuse to remove directories
	if info.IsDir() {
		fmt.Fprintf(stderr, "%s: cannot remove '%s': Is a directory\n",
			progName, path)
		return fmt.Errorf("is a directory")
	}
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
	fmt.Fprintln(w, "  -f, --force           ignore nonexistent files and arguments, never prompt")
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
