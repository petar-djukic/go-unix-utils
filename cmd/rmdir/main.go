// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/rmdir implements GNU rmdir: remove empty directories.
//
// Implements prd035-rmdir R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "rmdir"

type options struct {
	parents            bool
	ignoreFailNonEmpty bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run removes each directory specified as a positional argument.
// R1.2, R2.3: processes each directory independently.
func run(args []string, stderr *os.File) int {
	opts, dirs := parseArgs(args)
	if len(dirs) == 0 {
		fmt.Fprintln(stderr, "rmdir: missing operand")                   //nolint:errcheck
		fmt.Fprintln(stderr, "Try 'rmdir --help' for more information.") //nolint:errcheck
		return 1
	}
	exitCode := 0
	for _, dir := range dirs {
		if err := removeDir(dir); err != nil {
			if !shouldSuppress(err, opts) {
				reportError(stderr, dir, err)
				exitCode = 1
			}
			continue
		}
		// R2.1: ascend through parent directories when -p is set.
		if opts.parents {
			if err := removeParents(dir, stderr, opts); err != nil {
				exitCode = 1
			}
		}
	}
	return exitCode
}

// parseArgs extracts flags and positional directory arguments.
func parseArgs(args []string) (options, []string) {
	var opts options
	var dirs []string
	endOfFlags := false
	for i := range len(args) {
		a := args[i]
		if endOfFlags || !strings.HasPrefix(a, "-") || a == "-" {
			dirs = append(dirs, a)
			continue
		}
		if a == "--" {
			endOfFlags = true
			continue
		}
		if a == "--parents" {
			opts.parents = true
			continue
		}
		if a == "--ignore-fail-on-non-empty" {
			opts.ignoreFailNonEmpty = true
			continue
		}
		if strings.HasPrefix(a, "--") {
			continue
		}
		// Short flags: iterate characters after '-'.
		for _, ch := range a[1:] {
			if ch == 'p' {
				opts.parents = true
			}
		}
	}
	return opts, dirs
}

// removeDir removes a single empty directory.
// R1.1: removes empty directory via os.Remove.
// R1.3: os.Remove fails on non-empty directories.
// R1.4: fails on non-existent or non-directory targets.
func removeDir(dir string) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return &os.PathError{Op: "remove", Path: dir, Err: fmt.Errorf("Not a directory")}
	}
	return os.Remove(dir)
}

// removeParents removes each successive empty parent component of dir.
// R2.1: ascends through parent directories after removing the target.
// R2.2: stops when a parent removal fails.
func removeParents(dir string, stderr *os.File, opts options) error {
	parent := filepath.Dir(filepath.Clean(dir))
	for parent != "." && parent != "/" {
		if err := os.Remove(parent); err != nil {
			if !shouldSuppress(err, opts) {
				reportParentError(stderr, parent, err)
				return err
			}
			return nil
		}
		parent = filepath.Dir(parent)
	}
	return nil
}

// shouldSuppress returns true if the error should be silenced.
// R3.1: --ignore-fail-on-non-empty suppresses non-empty errors.
func shouldSuppress(err error, opts options) bool {
	return opts.ignoreFailNonEmpty && isNonEmptyError(err)
}

// isNonEmptyError reports whether err indicates a non-empty directory.
func isNonEmptyError(err error) bool {
	return errors.Is(err, syscall.ENOTEMPTY)
}

// extractReason returns the inner error message from a PathError.
func extractReason(err error) string {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// reportError writes an rmdir error to stderr in GNU format.
func reportError(stderr *os.File, dir string, err error) {
	fmt.Fprintf(stderr, "%s: failed to remove '%s': %s\n", progName, dir, extractReason(err)) //nolint:errcheck
}

// reportParentError writes a -p parent removal error in GNU format.
func reportParentError(stderr *os.File, dir string, err error) {
	fmt.Fprintf(stderr, "%s: failed to remove directory '%s': %s\n", progName, dir, extractReason(err)) //nolint:errcheck
}
