// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements du: recursive directory disk usage reporting.
// Implements srd009-du R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3.
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
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
// R1.5: processes multiple arguments in order, each independently.
// R4.1: exits 0 on success. R4.2: exits 1 on any error.
func run() int {
	opts, paths := parseArgs()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	w := &walker{
		opts: opts,
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

// parseArgs extracts flags and paths from command-line arguments.
// R2.1: -h for human-readable. R2.2: -s for summary.
// R2.3: -a for all files. R2.5: -k accepted as no-op.
func parseArgs() (options, []string) {
	opts := options{useKBlocks: true, maxDepth: -1}
	var paths []string
	for _, arg := range os.Args[1:] {
		if parseFlag(arg, &opts) {
			continue
		}
		paths = append(paths, arg)
	}
	return opts, paths
}

// parseFlag handles a single flag argument. Returns true if consumed.
func parseFlag(arg string, opts *options) bool {
	switch {
	case arg == "-k":
		return true
	case arg == "-h" || arg == "--human-readable":
		opts.humanReadable = true
		return true
	case arg == "-s" || arg == "--summarize":
		opts.summary = true
		return true
	case arg == "-a" || arg == "--all":
		opts.allFiles = true
		return true
	case strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--"):
		return parseCombinedFlags(arg[1:], opts)
	}
	return false
}

// parseCombinedFlags handles combined short flags like -hs, -ak.
func parseCombinedFlags(flags string, opts *options) bool {
	for _, c := range flags {
		switch c {
		case 'k':
			// no-op
		case 'h':
			opts.humanReadable = true
		case 's':
			opts.summary = true
		case 'a':
			opts.allFiles = true
		default:
			return false
		}
	}
	return true
}

// processArg handles a single command-line argument.
// R1.2: reports disk usage for each path argument.
// R1.5: each argument is traversed independently.
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
// R2.2: when -s, only the top-level (depth 0) entry is printed.
func (w *walker) walkDir(path string, fi *sys.FileInfo, depth int) int64 {
	total := fi.Blocks
	entries, err := os.ReadDir(path)
	if err != nil {
		w.reportError(path, err)
		if !w.opts.summary || depth == 0 {
			w.printEntry(total, path)
		}
		return total
	}
	for _, e := range entries {
		childPath := joinPath(path, e.Name())
		total += w.walkChild(childPath, depth+1)
	}
	if w.shouldPrintDir(depth) {
		w.printEntry(total, path)
	}
	return total
}

// shouldPrintDir decides whether to print a directory entry.
// R2.2: -s suppresses all output except top-level (depth 0).
func (w *walker) shouldPrintDir(depth int) bool {
	if w.opts.summary {
		return depth == 0
	}
	return true
}

// walkChild processes a single entry during directory traversal.
// R1.4: uses Lstat so symbolic links are not followed.
// R2.3: -a prints a line for every file, not just directories.
func (w *walker) walkChild(path string, depth int) int64 {
	fi, err := sys.Lstat(path)
	if err != nil {
		w.reportError(path, err)
		return 0
	}
	if fi.Mode.IsDir() {
		return w.walkDir(path, fi, depth)
	}
	blocks := w.fileBlocks(fi)
	if w.opts.allFiles && !w.opts.summary {
		w.printEntry(blocks, path)
	}
	return blocks
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
// R2.1: -h uses pkg/format.HumanSize with binary units.
func (w *walker) formatSize(blocks512 int64) string {
	if w.opts.humanReadable {
		// R2.1: convert 512-byte blocks to bytes for HumanSize.
		bytes := blocks512 * 512
		return format.HumanSize(bytes, format.HumanSizeOpts{Binary: true})
	}
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
	fmt.Fprintf(os.Stderr, "du: cannot access '%s': %s\n", path, osErrorMessage(err))
	w.hasError = true
}

// osErrorMessage extracts the underlying OS error message from a Go
// error, matching GNU coreutils strerror(errno) output style.
func osErrorMessage(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return capitalizeFirst(errno.Error())
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return capitalizeFirst(pathErr.Err.Error())
	}
	return err.Error()
}

// capitalizeFirst returns s with the first rune uppercased.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
