// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/du implements recursive directory disk usage reporting.
// Implements srd009-du R1.1-R1.5, R2.1-R2.8, R3.1-R3.3, R4.1-R4.2, R5.1.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type inodeKey struct {
	Dev uint64
	Ino uint64
}

type options struct {
	summarize     bool
	allFiles      bool
	grandTotal    bool
	maxDepth      int
	megaBlocks    bool
	oneFileSystem bool
	apparentSize  bool
	humanReadable bool
	siUnits       bool
	nullTerm      bool
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
		printEntry(cumTotal, "total", &opts)
	}

	os.Exit(exitCode)
}

func parseFlags() options {
	var opts options
	flag.BoolVar(&opts.summarize, "s", false, "display only a total for each argument")
	flag.BoolVar(&opts.allFiles, "a", false, "write counts for all files, not just directories")
	flag.BoolVar(&opts.grandTotal, "c", false, "produce a grand total")
	flag.IntVar(&opts.maxDepth, "d", -1, "max display depth")
	flag.IntVar(&opts.maxDepth, "max-depth", -1, "max display depth")
	flag.Bool("k", false, "use 1024-byte blocks (default)")
	flag.BoolVar(&opts.megaBlocks, "m", false, "use 1048576-byte blocks")
	flag.BoolVar(&opts.oneFileSystem, "x", false, "skip directories on different file systems")
	flag.BoolVar(&opts.oneFileSystem, "one-file-system", false, "skip directories on different file systems")
	flag.BoolVar(&opts.apparentSize, "apparent-size", false, "print apparent sizes rather than disk usage")
	flag.BoolVar(&opts.humanReadable, "h", false, "print sizes in human readable format")
	flag.BoolVar(&opts.siUnits, "si", false, "like -h, but use powers of 1000 not 1024")
	flag.BoolVar(&opts.nullTerm, "0", false, "end each output line with NUL, not newline")
	flag.BoolVar(&opts.nullTerm, "null", false, "end each output line with NUL, not newline")
	flag.Parse()
	return opts
}

func processArg(path string, seen map[inodeKey]bool, opts *options) (int64, error) {
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %v\n", err)
		return 0, err
	}

	rootDev := fi.Dev

	if !fi.Mode.IsDir() {
		size := fileSize(fi, seen, opts)
		printEntry(size, path, opts)
		return size, nil
	}

	return walkDir(path, fi, seen, opts, 0, rootDev)
}

func walkDir(path string, fi *sys.FileInfo, seen map[inodeKey]bool, opts *options, depth int, rootDev uint64) (int64, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %v\n", err)
		size := sizeContrib(fi, opts)
		if shouldPrint(depth, true, opts) {
			printEntry(size, path, opts)
		}
		return size, err
	}

	total, firstErr := accumChildren(path, entries, seen, opts, depth, rootDev)
	total += sizeContrib(fi, opts)
	if shouldPrint(depth, true, opts) {
		printEntry(total, path, opts)
	}
	return total, firstErr
}

func accumChildren(dir string, entries []os.DirEntry, seen map[inodeKey]bool, opts *options, parentDepth int, rootDev uint64) (int64, error) {
	var total int64
	var firstErr error
	childDepth := parentDepth + 1

	for _, entry := range entries {
		childPath := filepath.Join(dir, entry.Name())
		size, err := childSize(childPath, seen, opts, childDepth, rootDev)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		total += size
	}

	return total, firstErr
}

func childSize(path string, seen map[inodeKey]bool, opts *options, depth int, rootDev uint64) (int64, error) {
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %v\n", err)
		return 0, err
	}

	if fi.Mode.IsDir() {
		if opts.oneFileSystem && fi.Dev != rootDev {
			return 0, nil
		}
		return walkDir(path, fi, seen, opts, depth, rootDev)
	}

	size := fileSize(fi, seen, opts)
	if shouldPrint(depth, false, opts) {
		printEntry(size, path, opts)
	}
	return size, nil
}

func shouldPrint(depth int, isDir bool, opts *options) bool {
	if opts.summarize {
		return depth == 0
	}
	if opts.maxDepth >= 0 && depth > opts.maxDepth {
		return false
	}
	if isDir {
		return true
	}
	return opts.allFiles
}

func fileSize(fi *sys.FileInfo, seen map[inodeKey]bool, opts *options) int64 {
	key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
	if seen[key] {
		return 0
	}
	seen[key] = true
	return sizeContrib(fi, opts)
}

func sizeContrib(fi *sys.FileInfo, opts *options) int64 {
	if opts.apparentSize {
		return fi.Size
	}
	return fi.Blocks
}

func displaySize(raw int64, opts *options) int64 {
	if opts.apparentSize {
		if opts.megaBlocks {
			return (raw + 1048575) / 1048576
		}
		return (raw + 1023) / 1024
	}
	if opts.megaBlocks {
		return (raw + 2047) / 2048
	}
	return raw / 2
}

func printEntry(size int64, path string, opts *options) {
	var sizeStr string
	if opts.humanReadable || opts.siUnits {
		bytes := size
		if !opts.apparentSize {
			bytes = size * 512
		}
		sizeStr = format.HumanSize(bytes, format.HumanSizeOpts{Binary: !opts.siUnits})
	} else {
		sizeStr = fmt.Sprintf("%d", displaySize(size, opts))
	}
	terminator := "\n"
	if opts.nullTerm {
		terminator = "\x00"
	}
	fmt.Fprintf(os.Stdout, "%s\t%s%s", sizeStr, path, terminator)
}
