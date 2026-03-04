// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the du utility for reporting recursive directory
// disk usage.
//
// Implements prd009-du (R1.1, R1.2, R1.3, R1.4, R1.5, R2.2, R2.3, R2.4, R2.7, R4.1, R4.2, R5.1).
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
type options struct {
	// R2.3: -a prints a size line for every file, not just directories.
	allFiles bool
	// R2.2: -s prints only one total line per argument.
	summaryOnly bool
	// R2.7: -c prints a grand total line after all arguments.
	grandTotal bool
	// R2.4: -d N limits the depth of reported entries. -1 means unlimited.
	maxDepth int
}

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

// duArg processes a single command-line argument. Returns the total raw
// 512-byte block count for the argument and any error encountered.
// R1.1: reads named paths in argument order.
// R1.5: each argument is traversed independently.
func duArg(arg string, opts *options) (int64, error) {
	info, err := os.Lstat(arg)
	if err != nil {
		reportError(arg, err)
		return 0, err
	}

	if !info.IsDir() {
		// Non-directory argument: print its own block count as a single line.
		meta, err := sys.Lstat(arg)
		if err != nil {
			reportError(arg, err)
			return 0, err
		}
		blocks := meta.Blocks()
		fmt.Printf("%d\t%s\n", displayBlocks(blocks), arg)
		return blocks, nil
	}

	// Directory: depth-first traversal printing per-directory subtotals.
	totalBlocks, hasErr := walkDir(arg, opts, 0)
	if hasErr {
		return totalBlocks, fmt.Errorf("errors during traversal of %s", arg)
	}
	return totalBlocks, nil
}

// walkDir recursively traverses a directory, accumulating 512-byte block
// counts bottom-up. The depth parameter tracks how deep we are below the
// command-line argument (0 = the argument itself). Returns the total raw
// block count for the subtree and whether any error occurred during traversal.
// R1.1: recurse into each directory and print accumulated size.
// R1.4: does not follow symbolic links (uses sys.Lstat and DirEntry.IsDir).
// R2.3: when -a is active, prints non-directory files.
// R2.2: when -s is active, suppresses all output except the argument total.
// R2.4: when -d N is active, suppresses output for entries deeper than N.
func walkDir(dirPath string, opts *options, depth int) (int64, bool) {
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
			subtotal, childErr := walkDir(childPath, opts, depth+1)
			totalBlocks += subtotal
			if childErr {
				hasErr = true
			}
		} else {
			childMeta, err := sys.Lstat(childPath)
			if err != nil {
				reportError(childPath, err)
				hasErr = true
				continue
			}
			totalBlocks += childMeta.Blocks()
			// R2.3: when -a is active, print non-directory file entries.
			if opts.allFiles && shouldPrint(opts, depth+1) {
				fmt.Printf("%d\t%s\n", displayBlocks(childMeta.Blocks()), childPath)
			}
		}
	}

	// R1.3: print SIZE\tPATH for this directory.
	// R1.2: convert from 512-byte blocks to 1024-byte display units.
	// R2.2: when -s is active, only the argument-level total (depth 0) is printed by duArg.
	// R2.4: when -d N is active, only directories at depth <= N are printed.
	if shouldPrint(opts, depth) {
		fmt.Printf("%d\t%s\n", displayBlocks(totalBlocks), dirPath)
	}
	return totalBlocks, hasErr
}

// shouldPrint returns true if an entry at the given depth should be printed.
// R2.2: -s suppresses all output except the argument-level total (depth 0).
// R2.4: -d N suppresses output for entries deeper than N levels.
func shouldPrint(opts *options, depth int) bool {
	if opts.summaryOnly {
		return depth == 0
	}
	if opts.maxDepth >= 0 && depth > opts.maxDepth {
		return false
	}
	return true
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
	// R2.3: -a prints a size line for every file encountered.
	flag.BoolVar(&opts.allFiles, "a", false, "write counts for all files, not just directories")
	// R2.2: -s displays only a grand total for each argument.
	flag.BoolVar(&opts.summaryOnly, "s", false, "display only a total for each argument")
	// R2.7: -c produces a grand total after all arguments.
	flag.BoolVar(&opts.grandTotal, "c", false, "produce a grand total")
	// R2.4: -d N limits the depth of reported entries. Default -1 means unlimited.
	flag.IntVar(&opts.maxDepth, "d", -1, "print the total for a directory only if it is N or fewer levels below the command line argument")
	flag.Parse()

	// R1.5: process arguments in command-line order; default to ".".
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	exitCode := 0
	var grandTotalBlocks int64
	for _, arg := range args {
		blocks, err := duArg(arg, &opts)
		grandTotalBlocks += blocks
		if err != nil {
			exitCode = 1
		}
	}

	// R2.7: print grand total line when -c is active.
	if opts.grandTotal {
		fmt.Printf("%d\ttotal\n", displayBlocks(grandTotalBlocks))
	}

	os.Exit(exitCode)
}
