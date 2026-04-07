// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements du: recursive directory disk usage reporting.
// Implements srd009-du R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// inode uniquely identifies a file by device and inode number for
// hard-link deduplication (R3.1, R3.2).
type inode struct {
	Dev uint64
	Ino uint64
}

// options holds all parsed du flags from the SRD (R2.1-R2.8).
type options struct {
	humanReadable bool // -h: human-readable output (R2.1)
	summary       bool // -s: print only total per argument (R2.2)
	allFiles      bool // -a: print sizes for all files (R2.3)
	maxDepth      int  // -d N / --max-depth=N: limit depth (R2.4)
	useKBlocks    bool // -k: 1024-byte blocks (R2.5, default)
	useMBlocks    bool // -m: 1M blocks (R2.6)
	grandTotal    bool // -c: print grand total (R2.7)
	apparentSize  bool // --apparent-size: use st_size (R2.8)
	hasMaxDepth   bool // whether -d was explicitly set
}

// walker holds traversal state for a single du invocation.
type walker struct {
	opts     options
	seen     map[inode]bool // hard-link deduplication (R3.1, R3.3)
	hasError bool           // R4.2: set when any error occurs
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run())
}

// run parses arguments, walks each path, and returns an exit code.
// R1.1: defaults to "." when no arguments given.
// R4.1: exits 0 on success. R4.2: exits 1 on any error.
func run() int {
	paths := parseArgs()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	w := &walker{
		opts: options{useKBlocks: true, maxDepth: -1},
		seen: make(map[inode]bool),
	}
	for _, p := range paths {
		w.processArg(p)
	}
	if w.hasError {
		return 1
	}
	return 0
}

// parseArgs extracts paths from command-line arguments.
// R2.5: accepts -k as a no-op (1K blocks is the default).
func parseArgs() []string {
	var paths []string
	for _, arg := range os.Args[1:] {
		if arg == "-k" {
			continue
		}
		paths = append(paths, arg)
	}
	return paths
}

// processArg handles a single command-line argument.
// R1.2: reports disk usage for each path argument.
func (w *walker) processArg(path string) {
	fi, err := sys.Lstat(path)
	if err != nil {
		w.reportError(path, err)
		return
	}
	if fi.Mode.IsDir() {
		w.walkDir(path, fi, 0)
		return
	}
	w.printEntry(w.fileBlocks(fi), path)
}

// walkDir reads a directory, recurses into children, and prints the
// accumulated size. R1.1: prints one line per subdirectory.
// R1.3: format is "SIZE\tPATH\n".
func (w *walker) walkDir(path string, fi *sys.FileInfo, depth int) int64 {
	total := fi.Blocks
	entries, err := os.ReadDir(path)
	if err != nil {
		w.reportError(path, err)
		w.printEntry(total, path)
		return total
	}
	for _, e := range entries {
		childPath := joinPath(path, e.Name())
		total += w.walkChild(childPath, depth+1)
	}
	w.printEntry(total, path)
	return total
}

// walkChild processes a single entry during directory traversal.
// R1.4: uses Lstat so symbolic links are not followed.
func (w *walker) walkChild(path string, depth int) int64 {
	fi, err := sys.Lstat(path)
	if err != nil {
		w.reportError(path, err)
		return 0
	}
	if fi.Mode.IsDir() {
		return w.walkDir(path, fi, depth)
	}
	return w.fileBlocks(fi)
}

// fileBlocks returns the 512-byte block count for a file, applying
// hard-link deduplication. R3.1: files with the same dev+ino are
// counted only once. R3.3: dedup is per invocation, not per argument.
func (w *walker) fileBlocks(fi *sys.FileInfo) int64 {
	key := inode{Dev: fi.Dev, Ino: fi.Ino}
	if w.seen[key] {
		return 0
	}
	if fi.Nlink > 1 {
		w.seen[key] = true
	}
	return fi.Blocks
}

// formatSize converts a 512-byte block count to the display unit.
// R1.2: default is 1024-byte (1K) blocks = ceil(blocks512 / 2).
func (w *walker) formatSize(blocks512 int64) string {
	kblocks := (blocks512 + 1) / 2
	return fmt.Sprintf("%d", kblocks)
}

// printEntry prints one output line in "SIZE\tPATH\n" format.
// R1.3: SIZE is the formatted size, PATH is the entry path.
func (w *walker) printEntry(blocks512 int64, path string) {
	fmt.Fprintf(os.Stdout, "%s\t%s\n", w.formatSize(blocks512), path)
}

// joinPath concatenates a parent directory and child name, preserving
// the parent path prefix. Uses string concatenation rather than
// filepath.Join to preserve "./" prefix matching GNU du behavior.
func joinPath(parent, child string) string {
	return parent + "/" + child
}

// reportError prints a diagnostic to stderr and sets the error flag.
// R4.2: prints a diagnostic for each error and continues processing.
func (w *walker) reportError(path string, err error) {
	fmt.Fprintf(os.Stderr, "du: cannot access '%s': %v\n", path, err)
	w.hasError = true
}
