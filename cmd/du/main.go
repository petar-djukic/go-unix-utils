// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the du command, which estimates file space usage
// by recursively accumulating disk block counts for directories and files.
//
// Implements: prd009-du R1, R2, R3, R5
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// options holds the parsed flag configuration for a du invocation.
type options struct {
	humanReadable bool // -h: human-readable output via pkg/format.HumanSize
	apparentSize  bool // --apparent-size: use st_size instead of st_blocks
	bytesMode     bool // -b: apparent size displayed in raw bytes
	total         bool // -c: print grand total after all arguments
	all           bool // -a: write counts for all files, not just directories
	maxDepth      int  // -d N: limit reported depth (-1 means unlimited)
}

// devIno is a deduplication key combining device and inode numbers (R3.1).
type devIno struct {
	Dev uint64
	Ino uint64
}

// run parses flags and processes inputs. Returns 0 on success, 1 on error.
//
// Implements: prd009-du R1, R2, R4, R5
func run(args []string) int {
	installSIGPIPEHandler()

	fs := flag.NewFlagSet("du", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	opts := &options{maxDepth: -1}

	fs.BoolVar(&opts.humanReadable, "h", false, "print sizes in human readable format")
	fs.BoolVar(&opts.apparentSize, "apparent-size", false, "print apparent sizes rather than disk usage")
	fs.BoolVar(&opts.bytesMode, "b", false, "equivalent to --apparent-size, in bytes")

	var summarize bool
	fs.BoolVar(&summarize, "s", false, "display only a total for each argument")
	fs.BoolVar(&summarize, "summarize", false, "display only a total for each argument")

	fs.BoolVar(&opts.total, "c", false, "produce a grand total")
	fs.BoolVar(&opts.total, "total", false, "produce a grand total")

	fs.BoolVar(&opts.all, "a", false, "write counts for all files, not just directories")
	fs.BoolVar(&opts.all, "all", false, "write counts for all files, not just directories")

	fs.IntVar(&opts.maxDepth, "d", -1, "max display depth")
	fs.IntVar(&opts.maxDepth, "max-depth", -1, "max display depth")

	// -k is accepted but is the default (prd009-du R2.5).
	fs.Bool("k", false, "like --block-size=1K")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// -b implies --apparent-size with byte display.
	if opts.bytesMode {
		opts.apparentSize = true
	}

	// -s is equivalent to -d 0 (prd009-du R2.2).
	if summarize {
		opts.maxDepth = 0
	}

	// Default to current directory when no arguments given (R1.1).
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	exitCode := 0
	var grandTotal int64
	seen := make(map[devIno]bool)

	for _, path := range paths {
		size, errFlag := walkPath(path, opts, 0, seen)
		if errFlag != 0 {
			exitCode = 1
		}
		grandTotal += size
	}

	// Grand total line (R2.7).
	if opts.total {
		printSize(grandTotal, "total", opts)
	}

	return exitCode
}

// walkPath recursively traverses a path depth-first and returns the total
// size in bytes. Directories are printed after their children (depth-first
// order). Returns (totalBytes, errorFlag) where errorFlag is 0 or 1.
//
// Implements: prd009-du R1.1, R1.3, R1.4, R1.5
func walkPath(path string, opts *options, depth int, seen map[devIno]bool) (int64, int) {
	fi, err := sys.Lstat(path)
	if err != nil {
		printError(path, err)
		return 0, 1
	}

	// Non-directory entry.
	if !fi.Mode.IsDir() {
		size := entrySize(fi, opts, seen)
		if depth == 0 || (opts.all && (opts.maxDepth < 0 || depth <= opts.maxDepth)) {
			printSize(size, path, opts)
		}
		return size, 0
	}

	// Directory: read children and recurse.
	entries, err := os.ReadDir(path)
	if err != nil {
		printError(path, err)
		// Still count this directory's own allocation.
		size := entrySize(fi, opts, seen)
		if opts.maxDepth < 0 || depth <= opts.maxDepth {
			printSize(size, path, opts)
		}
		return size, 1
	}

	exitCode := 0
	total := entrySize(fi, opts, seen)

	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())
		childSize, childErr := walkPath(childPath, opts, depth+1, seen)
		total += childSize
		if childErr != 0 {
			exitCode = 1
		}
	}

	// Print directory total after children (depth-first order).
	if opts.maxDepth < 0 || depth <= opts.maxDepth {
		printSize(total, path, opts)
	}

	return total, exitCode
}

// entrySize returns the size contribution of a single entry in bytes.
// Handles hard-link deduplication and apparent-size mode.
//
// Implements: prd009-du R1.2, R2.8, R3.1, R3.2, R3.3
func entrySize(fi *sys.FileInfo, opts *options, seen map[devIno]bool) int64 {
	// Hard-link deduplication: files with nlink > 1 are tracked by (dev, ino).
	if fi.Nlink > 1 && !fi.Mode.IsDir() {
		key := devIno{Dev: fi.Dev, Ino: fi.Ino}
		if seen[key] {
			return 0
		}
		seen[key] = true
	}

	if opts.apparentSize {
		return fi.Size
	}
	return fi.Blocks * 512
}

// printSize formats and prints a size/path line to stdout.
//
// Implements: prd009-du R1.3, R2.1
func printSize(bytes int64, path string, opts *options) {
	var sizeStr string
	switch {
	case opts.humanReadable:
		sizeStr = format.HumanSize(bytes, format.HumanSizeOpts{Binary: true})
	case opts.bytesMode:
		sizeStr = fmt.Sprintf("%d", bytes)
	default:
		// 1024-byte blocks with ceiling division.
		blocks := (bytes + 1023) / 1024
		sizeStr = fmt.Sprintf("%d", blocks)
	}
	fmt.Printf("%s\t%s\n", sizeStr, path)
}

// installSIGPIPEHandler sets up a signal handler for SIGPIPE that exits
// cleanly, preventing non-zero exit when stdout is closed by a downstream
// consumer.
//
// Implements: prd009-du R5.1
func installSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}

// printError writes a file access error to stderr in the format
// "du: <path>: <message>".
//
// Implements: prd009-du R4.2
func printError(path string, err error) {
	if pe, ok := err.(*os.PathError); ok {
		fmt.Fprintf(os.Stderr, "du: %s: %s\n", pe.Path, pe.Err)
	} else {
		fmt.Fprintf(os.Stderr, "du: %s: %v\n", path, err)
	}
}
