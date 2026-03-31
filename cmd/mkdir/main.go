// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/mkdir implements GNU mkdir: create directories.
//
// Implements prd034-mkdir R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3, R3.4.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "mkdir"

type options struct {
	parents bool
	verbose bool
	mode    string // octal mode string; empty means default (R3.2)
	dirs    []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// parseArgs parses command-line arguments into options.
func parseArgs(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			opts.dirs = append(opts.dirs, args[i+1:]...)
			return opts, nil
		}
		needsNext, err := parseArg(arg, &opts)
		if err != nil {
			return opts, err
		}
		if needsNext {
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("option requires an argument -- 'm'")
			}
			opts.mode = args[i]
		}
	}
	return opts, nil
}

// parseArg processes a single argument. Returns true if the next
// argument should be consumed as a mode value.
func parseArg(arg string, opts *options) (bool, error) {
	switch {
	case arg == "--parents":
		opts.parents = true
	case arg == "--verbose":
		opts.verbose = true
	case arg == "--mode":
		return true, nil
	case strings.HasPrefix(arg, "--mode="):
		opts.mode = arg[len("--mode="):]
	case strings.HasPrefix(arg, "--"):
		return false, fmt.Errorf("unrecognized option '%s'", arg)
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		return parseShortFlags(arg[1:], opts)
	default:
		opts.dirs = append(opts.dirs, arg)
	}
	return false, nil
}

// parseShortFlags processes bundled short flags like -pv or -pm0755.
// Returns true if the next argument should be consumed as a mode value.
func parseShortFlags(flags string, opts *options) (bool, error) {
	for i, ch := range flags {
		switch ch {
		case 'p':
			opts.parents = true
		case 'v':
			opts.verbose = true
		case 'm':
			rest := flags[i+1:]
			if rest != "" {
				opts.mode = rest
				return false, nil
			}
			return true, nil
		default:
			return false, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return false, nil
}

// parseMode converts an octal mode string to os.FileMode.
func parseMode(s string) (os.FileMode, error) {
	val, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode: %q", s)
	}
	return os.FileMode(val), nil
}

// run creates directories specified as positional arguments.
// R1.2: processes each directory independently.
// R1.3: reports errors per directory without aborting remaining arguments.
func run(args []string, stdout, stderr *os.File) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)                          //nolint:errcheck
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName) //nolint:errcheck
		return 1
	}
	if len(opts.dirs) == 0 {
		fmt.Fprintln(stderr, "mkdir: missing operand")                   //nolint:errcheck
		fmt.Fprintln(stderr, "Try 'mkdir --help' for more information.") //nolint:errcheck
		return 1
	}
	exitCode := 0
	for _, dir := range opts.dirs {
		if err := createDir(dir, opts, stdout); err != nil {
			reportError(stderr, dir, err)
			exitCode = 1
		}
	}
	return exitCode
}

// createDir creates a single directory, optionally with parents.
// R3.2: default permissions when -m not given.
// R3.3: -m applies only to final directory when combined with -p.
func createDir(dir string, opts options, stdout *os.File) error {
	if opts.parents {
		return mkdirParents(dir, opts, stdout)
	}
	return mkdirSingle(dir, opts, stdout)
}

// mkdirSingle creates a single directory without parents.
func mkdirSingle(dir string, opts options, stdout *os.File) error {
	if err := os.Mkdir(dir, 0o777); err != nil {
		return err
	}
	if err := applyMode(dir, opts.mode); err != nil {
		return err
	}
	if opts.verbose {
		printCreated(stdout, dir)
	}
	return nil
}

// applyMode sets the mode on a directory if a mode string is specified.
// R3.1: explicit -m mode overrides umask.
// R3.2: when mode is empty, default permissions apply (no-op).
func applyMode(dir, mode string) error {
	if mode == "" {
		return nil
	}
	parsed, err := parseMode(mode)
	if err != nil {
		return err
	}
	return os.Chmod(dir, parsed)
}

// mkdirParents creates dir and any missing parents.
// R2.1: create intermediate directories as needed.
// R2.2: no error if target already exists.
// R3.3: intermediate directories use default permissions; mode applies to final only.
func mkdirParents(dir string, opts options, stdout *os.File) error {
	if opts.mode == "" && !opts.verbose {
		return os.MkdirAll(dir, 0o777)
	}
	created, err := mkdirAllVerbose(dir, opts.verbose, stdout)
	if err != nil {
		return err
	}
	if created {
		return applyMode(dir, opts.mode)
	}
	return nil
}

// mkdirAllVerbose creates dir and parents, optionally printing each.
// Returns true if the final path was newly created.
func mkdirAllVerbose(path string, verbose bool, stdout *os.File) (bool, error) {
	if isDir(path) {
		return false, nil
	}
	parent := filepath.Dir(path)
	if parent != path {
		if _, err := mkdirAllVerbose(parent, verbose, stdout); err != nil {
			return false, err
		}
	}
	if err := os.Mkdir(path, 0o777); err != nil {
		if isDir(path) {
			return false, nil
		}
		return false, err
	}
	if verbose {
		printCreated(stdout, path)
	}
	return true, nil
}

// isDir reports whether path exists and is a directory.
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// printCreated prints the GNU-format verbose message for a created directory.
func printCreated(stdout *os.File, dir string) {
	fmt.Fprintf(stdout, "%s: created directory '%s'\n", progName, dir) //nolint:errcheck
}

// reportError writes a mkdir error to stderr in GNU format.
func reportError(stderr *os.File, dir string, err error) {
	reason := err.Error()
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		reason = pe.Err.Error()
	}
	fmt.Fprintf(stderr, "%s: cannot create directory '%s': %s\n", progName, dir, reason) //nolint:errcheck
}
