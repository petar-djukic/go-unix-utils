// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/mkdir implements GNU mkdir: create directories.
//
// Implements prd034-mkdir R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.4.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "mkdir"

type options struct {
	parents bool
	verbose bool
	dirs    []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// parseArgs parses command-line arguments into options.
func parseArgs(args []string) (options, error) {
	var opts options
	for i := range len(args) {
		arg := args[i]
		if arg == "--" {
			opts.dirs = append(opts.dirs, args[i+1:]...)
			return opts, nil
		}
		if err := parseArg(arg, &opts); err != nil {
			return opts, err
		}
	}
	return opts, nil
}

// parseArg processes a single argument, updating opts.
func parseArg(arg string, opts *options) error {
	switch {
	case arg == "--parents":
		opts.parents = true
	case arg == "--verbose":
		opts.verbose = true
	case strings.HasPrefix(arg, "--"):
		return fmt.Errorf("unrecognized option '%s'", arg)
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		return parseShortFlags(arg[1:], opts)
	default:
		opts.dirs = append(opts.dirs, arg)
	}
	return nil
}

// parseShortFlags processes bundled short flags like -pv.
func parseShortFlags(flags string, opts *options) error {
	for _, ch := range flags {
		switch ch {
		case 'p':
			opts.parents = true
		case 'v':
			opts.verbose = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
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
func createDir(dir string, opts options, stdout *os.File) error {
	if opts.parents {
		return mkdirParents(dir, opts.verbose, stdout)
	}
	if err := os.Mkdir(dir, 0o777); err != nil {
		return err
	}
	if opts.verbose {
		printCreated(stdout, dir)
	}
	return nil
}

// mkdirParents creates dir and any missing parents.
// R2.1: create intermediate directories as needed.
// R2.2: no error if target already exists.
// R2.3: intermediate directories use default permissions.
func mkdirParents(dir string, verbose bool, stdout *os.File) error {
	if !verbose {
		return os.MkdirAll(dir, 0o777)
	}
	return mkdirAllVerbose(dir, stdout)
}

// mkdirAllVerbose creates dir and parents, printing each created directory.
// D3: prints a line for each intermediate directory created.
func mkdirAllVerbose(path string, stdout *os.File) error {
	if isDir(path) {
		return nil
	}
	parent := filepath.Dir(path)
	if parent != path {
		if err := mkdirAllVerbose(parent, stdout); err != nil {
			return err
		}
	}
	if err := os.Mkdir(path, 0o777); err != nil {
		if isDir(path) {
			return nil
		}
		return err
	}
	printCreated(stdout, path)
	return nil
}

// isDir reports whether path exists and is a directory.
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// printCreated prints the GNU-format verbose message for a created directory.
// D2: format is "mkdir: created directory 'NAME'".
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
