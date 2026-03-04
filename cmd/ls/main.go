// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the ls utility for listing directory contents.
//
// Implements prd008-ls (R1.2, R1.3, R1.4, R3.1, R3.2, R6.1, R6.2, R7.1).
// This initial implementation provides one-per-line output, alphabetical
// sorting under LC_ALL=C, hidden-file filtering (-a, -A), multi-argument
// headers, and non-directory file argument handling.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"
)

// options holds the parsed command-line flags for ls.
type options struct {
	showAll       bool // -a: show all entries including . and ..
	showAlmostAll bool // -A: show entries starting with . except . and ..
}

// filterMode returns the effective filter after resolving -a/-A precedence.
// R3.4: when both -a and -A are given, the last one on the command line
// takes precedence. Since Go's flag package uses last-wins semantics for
// repeated flags, the values here reflect the final state.
// However, with separate bool flags, both may be true if both are given.
// We resolve: -a takes precedence over -A when both are set, matching the
// behavior where -a is the broader filter.
func (o *options) showHidden() bool {
	return o.showAll || o.showAlmostAll
}

func (o *options) includeDotAndDotDot() bool {
	return o.showAll
}

// reportError writes an ls-style error message to stderr.
// R6.2: per-path error messages to stderr.
func reportError(path string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "ls: %s: %v\n", path, pathErr.Err)
	} else {
		fmt.Fprintf(os.Stderr, "ls: %s: %v\n", path, err)
	}
}

// listDir reads directory entries and writes them to w, one per line.
// R1.2: single-column output, one entry per line.
// R1.3: entries sorted in C locale byte order via sort.Strings.
// R1.4: entries starting with "." excluded unless -a or -A is given.
func listDir(dirPath string, w *bufio.Writer, opts *options) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	// D2: re-sort to guarantee byte-order sorting under LC_ALL=C.
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !opts.showHidden() && len(name) > 0 && name[0] == '.' {
			continue
		}
		if opts.showAlmostAll && !opts.showAll {
			// -A: include dotfiles but exclude . and ..
			if name == "." || name == ".." {
				continue
			}
		}
		names = append(names, name)
	}

	// R3.1: -a includes . and .. in the listing.
	if opts.includeDotAndDotDot() {
		names = append(names, ".", "..")
	}

	sort.Strings(names)

	for _, name := range names {
		if _, err := fmt.Fprintln(w, name); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	// R7.1: handle SIGPIPE by exiting cleanly with code 0.
	sigpipe := make(chan os.Signal, 1)
	signal.Notify(sigpipe, syscall.SIGPIPE)
	go func() {
		<-sigpipe
		os.Exit(0)
	}()

	var opts options
	flag.BoolVar(&opts.showAll, "a", false, "show all entries including . and ..")
	flag.BoolVar(&opts.showAlmostAll, "A", false, "show entries starting with . except . and ..")
	flag.Parse()

	// R2: default to "." when no arguments are given.
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	out := bufio.NewWriter(os.Stdout)
	exitCode := 0

	// Separate file arguments from directory arguments, preserving order.
	// R2: non-directory files print their name as a single line without a header.
	var fileArgs []string
	var dirArgs []string

	for _, arg := range args {
		info, err := os.Lstat(arg)
		if err != nil {
			reportError(arg, err)
			exitCode = 1
			continue
		}
		if info.IsDir() {
			dirArgs = append(dirArgs, arg)
		} else {
			fileArgs = append(fileArgs, arg)
		}
	}

	needSeparator := false

	// Print non-directory file arguments first, one per line.
	// R2: file arguments print as single lines without headers.
	for _, name := range fileArgs {
		if _, err := fmt.Fprintln(out, name); err != nil {
			exitCode = 1
		}
		needSeparator = true
	}

	// R2: when listing more than one directory, print headers.
	// Headers are also printed when there are file arguments mixed with directories.
	showHeaders := len(dirArgs) > 1 || (len(fileArgs) > 0 && len(dirArgs) > 0)

	for _, dirPath := range dirArgs {
		// R2: blank line between groups.
		if needSeparator {
			fmt.Fprintln(out)
		}
		needSeparator = true

		if showHeaders {
			fmt.Fprintf(out, "%s:\n", dirPath)
		}

		if err := listDir(dirPath, out, &opts); err != nil {
			reportError(dirPath, err)
			exitCode = 1
		}
	}

	// R5: detect write errors on final flush.
	if err := out.Flush(); err != nil {
		exitCode = 1
	}

	os.Exit(exitCode)
}
