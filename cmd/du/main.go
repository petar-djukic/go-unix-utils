// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the du utility for reporting recursive directory
// disk usage.
//
// Implements prd009-du (R1.1, R1.2, R1.3, R1.4, R1.5, R4.1, R4.2, R5.1).
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// options holds the parsed command-line flags for du.
// D3: scaffolding for future flags (-a, -s, -h, -c, -k, -d, etc.).
type options struct{}

// reportError writes a du-style error message to stderr.
// R4.2: per-path error messages to stderr.
func reportError(path string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "du: %s: %v\n", path, pathErr.Err)
	} else {
		fmt.Fprintf(os.Stderr, "du: %s: %v\n", path, err)
	}
}

// duArg processes a single command-line argument.
// R1.1: reads named paths in argument order.
// R1.5: each argument is traversed independently.
func duArg(arg string, opts *options) error {
	info, err := os.Lstat(arg)
	if err != nil {
		reportError(arg, err)
		return err
	}

	if !info.IsDir() {
		// Non-directory argument: print its own block count as a single line.
		meta, err := sys.Lstat(arg)
		if err != nil {
			reportError(arg, err)
			return err
		}
		fmt.Printf("%d\t%s\n", displayBlocks(meta.Blocks()), arg)
		return nil
	}

	// Directory: depth-first traversal printing per-directory subtotals.
	_, hasErr := walkDir(arg, opts)
	if hasErr {
		return fmt.Errorf("errors during traversal of %s", arg)
	}
	return nil
}

// walkDir recursively traverses a directory, accumulating 512-byte block
// counts bottom-up. Returns the total raw block count for the subtree and
// whether any error occurred during traversal.
// R1.1: recurse into each directory and print accumulated size.
// R1.4: does not follow symbolic links (uses sys.Lstat and DirEntry.IsDir).
func walkDir(dirPath string, opts *options) (int64, bool) {
	var totalBlocks int64
	hasErr := false

	// D1: use pkg/sys.Lstat to obtain the directory's own 512-byte block count.
	meta, err := sys.Lstat(dirPath)
	if err != nil {
		reportError(dirPath, err)
		return 0, true
	}
	totalBlocks += meta.Blocks()

	// D1: use os.ReadDir for directory listing; DirEntry.IsDir() uses d_type
	// from the dirent to avoid a second stat call per entry.
	entries, readErr := os.ReadDir(dirPath)
	if readErr != nil {
		reportError(dirPath, readErr)
		hasErr = true
	}

	for _, entry := range entries {
		childPath := joinPath(dirPath, entry.Name())

		if entry.IsDir() {
			subtotal, childErr := walkDir(childPath, opts)
			totalBlocks += subtotal
			if childErr {
				hasErr = true
			}
		} else {
			// Non-directory: count blocks but do not print in default mode.
			childMeta, err := sys.Lstat(childPath)
			if err != nil {
				reportError(childPath, err)
				hasErr = true
				continue
			}
			totalBlocks += childMeta.Blocks()
		}
	}

	// R1.3: print SIZE\tPATH for this directory.
	// R1.2: convert from 512-byte blocks to 1024-byte display units.
	fmt.Printf("%d\t%s\n", displayBlocks(totalBlocks), dirPath)
	return totalBlocks, hasErr
}

// joinPath constructs a child path by appending name to a directory path,
// preserving the argument prefix as given on the command line. Unlike
// filepath.Join, this does not clean the path (filepath.Join strips "./"
// prefixes, which changes du output).
func joinPath(dir, name string) string {
	if dir[len(dir)-1] == '/' {
		return dir + name
	}
	return dir + "/" + name
}

// displayBlocks converts a 512-byte block count to 1024-byte display units
// using integer ceiling division: (rawBlocks + 1) / 2.
// R1.2: 1024-byte block units by default.
func displayBlocks(rawBlocks int64) int64 {
	return (rawBlocks + 1) / 2
}

func main() {
	// R5.1: handle SIGPIPE by exiting cleanly.
	sigpipe := make(chan os.Signal, 1)
	signal.Notify(sigpipe, syscall.SIGPIPE)
	go func() {
		<-sigpipe
		os.Exit(0)
	}()

	var opts options
	flag.Parse()

	// R1.5: process arguments in command-line order; default to ".".
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	exitCode := 0
	for _, arg := range args {
		if err := duArg(arg, &opts); err != nil {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}
