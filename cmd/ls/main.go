// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd008-ls R1.1-R1.4: basic directory listing with single-column
// output (non-TTY default), dotfile filtering, multi-directory headers, mixed
// file/directory argument handling, and error diagnostics. Installs SIGPIPE
// handler per ARCHITECTURE.yaml shared protocol.
package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU ls format.
const progName = "ls"

func main() {
	// D1: Install SIGPIPE handler for clean pipe exit.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// D4: No flag parsing in this task; flags are deferred to R1.5+ tasks.
	// Filter out "--" separator; treat all non-"--" args as paths.
	var paths []string
	for _, arg := range args {
		if arg == "--" {
			continue
		}
		paths = append(paths, arg)
	}

	// R1.1/R1.2: Default to current directory when no arguments given.
	if len(paths) == 0 {
		paths = []string{"."}
	}

	exitCode := 0

	// R1.3: Separate file arguments from directory arguments. Files are
	// listed first, then directories, matching GNU ls argument ordering.
	var files []string
	var dirs []string
	var errs []string

	for _, path := range paths {
		fi, err := os.Lstat(path)
		if err != nil {
			// R1.4: Print diagnostic to stderr for inaccessible arguments.
			fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %v\n", progName, path, unwrapPathError(err))
			exitCode = 2
			errs = append(errs, path)
			continue
		}
		if fi.IsDir() {
			dirs = append(dirs, path)
		} else {
			files = append(files, path)
		}
	}

	// R1.3: Print file arguments first, one per line.
	for _, f := range files {
		fmt.Println(f)
	}

	// R1.2: When multiple directories (or mix of files and directories),
	// print each directory name as a header before its contents.
	needHeader := len(dirs) > 1 || (len(files) > 0 && len(dirs) > 0)

	// R1.3: Blank line between file list and first directory when both present.
	needBlankBefore := len(files) > 0 && len(dirs) > 0

	for i, dir := range dirs {
		if needBlankBefore || (i > 0) {
			fmt.Println()
		}
		needBlankBefore = false

		if needHeader {
			fmt.Printf("%s:\n", dir)
		}

		if err := listDir(dir); err != nil {
			fmt.Fprintf(os.Stderr, "%s: cannot open directory '%s': %v\n", progName, dir, unwrapPathError(err))
			exitCode = 2
		}
	}

	os.Exit(exitCode)
}

// listDir reads and prints the contents of a single directory.
// R1.1/R1.2: One entry per line (non-TTY default), sorted alphabetically.
// R1.4: Entries whose names start with "." are excluded by default.
func listDir(path string) error {
	// D2: os.ReadDir returns entries sorted by name, matching GNU ls
	// default sort order under LC_ALL=C.
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	// R1.3: Sort entries in C locale order. os.ReadDir already returns
	// sorted entries, but we sort explicitly to guarantee LC_ALL=C order.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		name := entry.Name()
		// R1.4: Skip dotfiles unless -a or -A is given (deferred to R2 tasks).
		if len(name) > 0 && name[0] == '.' {
			continue
		}
		fmt.Println(name)
	}

	return nil
}

// unwrapPathError extracts the inner error from *os.PathError for cleaner
// error messages matching GNU ls format.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
