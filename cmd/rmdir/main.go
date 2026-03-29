// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/rmdir implements GNU rmdir: remove empty directories.
//
// Implements prd035-rmdir R1.1, R1.2, R1.3, R1.4.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "rmdir"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run removes each directory specified as a positional argument.
// R1.2: processes each directory independently.
// R1.3, R1.4: reports errors per directory without aborting remaining.
func run(args []string, stderr *os.File) int {
	dirs := parseArgs(args)
	if len(dirs) == 0 {
		fmt.Fprintln(stderr, "rmdir: missing operand")                   //nolint:errcheck
		fmt.Fprintln(stderr, "Try 'rmdir --help' for more information.") //nolint:errcheck
		return 1
	}
	exitCode := 0
	for _, dir := range dirs {
		if err := removeDir(dir); err != nil {
			reportError(stderr, dir, err)
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs extracts positional directory arguments, handling --.
func parseArgs(args []string) []string {
	var dirs []string
	for i := range len(args) {
		if args[i] == "--" {
			dirs = append(dirs, args[i+1:]...)
			return dirs
		}
		dirs = append(dirs, args[i])
	}
	return dirs
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

// reportError writes an rmdir error to stderr in GNU format.
func reportError(stderr *os.File, dir string, err error) {
	reason := err.Error()
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		reason = pe.Err.Error()
	}
	fmt.Fprintf(stderr, "%s: failed to remove '%s': %s\n", progName, dir, reason) //nolint:errcheck
}
