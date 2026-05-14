// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/du implements recursive directory disk usage reporting.
// Implements srd009-du R1.1-R1.5, R2.2, R2.3, R2.7, R4.1-R4.2.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type inodeKey struct {
	Dev uint64
	Ino uint64
}

type options struct {
	summarize  bool
	allFiles   bool
	grandTotal bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts := parseFlags()
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	seen := make(map[inodeKey]bool)
	exitCode := 0
	var cumTotal int64

	for _, arg := range args {
		total, err := processArg(arg, seen, &opts)
		cumTotal += total
		if err != nil {
			exitCode = 1
		}
	}

	if opts.grandTotal {
		printEntry(cumTotal, "total")
	}

	os.Exit(exitCode)
}

func parseFlags() options {
	var opts options
	flag.BoolVar(&opts.summarize, "s", false, "display only a total for each argument")
	flag.BoolVar(&opts.allFiles, "a", false, "write counts for all files, not just directories")
	flag.BoolVar(&opts.grandTotal, "c", false, "produce a grand total")
	flag.Parse()
	return opts
}

func processArg(path string, seen map[inodeKey]bool, opts *options) (int64, error) {
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %v\n", err)
		return 0, err
	}

	if !fi.Mode.IsDir() {
		size := fileSize(fi, seen)
		printEntry(size, path)
		return size, nil
	}

	return walkDir(path, fi, seen, opts, 0)
}

func walkDir(path string, fi *sys.FileInfo, seen map[inodeKey]bool, opts *options, depth int) (int64, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %v\n", err)
		size := blockSize(fi)
		if shouldPrint(depth, true, opts) {
			printEntry(size, path)
		}
		return size, err
	}

	total, firstErr := accumChildren(path, entries, seen, opts, depth)
	total += blockSize(fi)
	if shouldPrint(depth, true, opts) {
		printEntry(total, path)
	}
	return total, firstErr
}

func accumChildren(dir string, entries []os.DirEntry, seen map[inodeKey]bool, opts *options, parentDepth int) (int64, error) {
	var total int64
	var firstErr error
	childDepth := parentDepth + 1

	for _, entry := range entries {
		childPath := filepath.Join(dir, entry.Name())
		size, err := childSize(childPath, seen, opts, childDepth)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		total += size
	}

	return total, firstErr
}

func childSize(path string, seen map[inodeKey]bool, opts *options, depth int) (int64, error) {
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %v\n", err)
		return 0, err
	}

	if fi.Mode.IsDir() {
		return walkDir(path, fi, seen, opts, depth)
	}

	size := fileSize(fi, seen)
	if shouldPrint(depth, false, opts) {
		printEntry(size, path)
	}
	return size, nil
}

func shouldPrint(depth int, isDir bool, opts *options) bool {
	if opts.summarize {
		return depth == 0
	}
	if isDir {
		return true
	}
	return opts.allFiles
}

func fileSize(fi *sys.FileInfo, seen map[inodeKey]bool) int64 {
	key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
	if seen[key] {
		return 0
	}
	seen[key] = true
	return blockSize(fi)
}

func blockSize(fi *sys.FileInfo) int64 {
	return fi.Blocks / 2
}

func printEntry(size int64, path string) {
	fmt.Fprintf(os.Stdout, "%d\t%s\n", size, path)
}
