// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements du: recursive directory disk usage reporting.
// Implements srd009-du R1.1-R1.5, R2.1-R2.8.
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
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

// options holds all parsed du flags (R2.1-R2.8).
type options struct {
	humanReadable bool // -h: human-readable output (R2.1)
	summary       bool // -s: print only total per argument (R2.2)
	allFiles      bool // -a: print sizes for all files (R2.3)
	maxDepth      int  // -d N / --max-depth=N: limit depth (R2.4)
	useMBlocks    bool // -m: 1M blocks (R2.6)
	grandTotal    bool // -c: print grand total (R2.7)
	apparentSize  bool // --apparent-size / -b: use st_size (R2.8)
	useBytes      bool // -b: display raw bytes
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
func run() int {
	opts, paths := parseArgs()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	w := &walker{
		opts: opts,
		seen: make(map[inode]bool),
	}
	var grandTotal int64
	for _, p := range paths {
		grandTotal += w.processArg(p)
	}
	// R2.7: -c prints a grand total line after all arguments.
	if w.opts.grandTotal {
		w.printEntry(grandTotal, "total")
	}
	if w.hasError {
		return 1
	}
	return 0
}

// parseArgs extracts flags and paths from command-line arguments.
func parseArgs() (options, []string) {
	opts := options{maxDepth: -1}
	var paths []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		consumed, extra := parseFlag(args[i], &opts, args[i+1:])
		if consumed {
			i += extra
			continue
		}
		paths = append(paths, args[i])
	}
	// R2.2: -s is equivalent to --max-depth=0.
	if opts.summary && !opts.hasMaxDepth {
		opts.maxDepth = 0
		opts.hasMaxDepth = true
	}
	return opts, paths
}

// parseFlag handles a single flag. Returns (consumed, extraArgsConsumed).
func parseFlag(arg string, opts *options, rest []string) (bool, int) {
	switch {
	case arg == "-k":
		return true, 0
	case arg == "-m":
		opts.useMBlocks = true
		return true, 0
	case arg == "-b" || arg == "--bytes":
		opts.useBytes = true
		opts.apparentSize = true
		return true, 0
	case arg == "-c" || arg == "--total":
		opts.grandTotal = true
		return true, 0
	case arg == "-h" || arg == "--human-readable":
		opts.humanReadable = true
		return true, 0
	case arg == "-s" || arg == "--summarize":
		opts.summary = true
		return true, 0
	case arg == "-a" || arg == "--all":
		opts.allFiles = true
		return true, 0
	case arg == "--apparent-size":
		opts.apparentSize = true
		return true, 0
	case strings.HasPrefix(arg, "--max-depth="):
		parseMaxDepthValue(arg[len("--max-depth="):], opts)
		return true, 0
	case arg == "--max-depth" || arg == "-d":
		return parseSeparateDepth(opts, rest)
	case strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--"):
		return parseCombinedFlags(arg[1:], opts, rest)
	}
	return false, 0
}

// parseSeparateDepth handles -d or --max-depth when the value is the next arg.
func parseSeparateDepth(opts *options, rest []string) (bool, int) {
	if len(rest) > 0 {
		parseMaxDepthValue(rest[0], opts)
		return true, 1
	}
	return true, 0
}

// parseMaxDepthValue parses a depth value string and sets opts.maxDepth.
func parseMaxDepthValue(val string, opts *options) {
	n, err := strconv.Atoi(val)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: invalid maximum depth '%s'\n", val)
		return
	}
	opts.maxDepth = n
	opts.hasMaxDepth = true
}

// parseCombinedFlags handles combined short flags like -hs, -ack.
func parseCombinedFlags(flags string, opts *options, rest []string) (bool, int) {
	for i := 0; i < len(flags); i++ {
		switch flags[i] {
		case 'k':
			// R2.5: no-op, default is 1K blocks
		case 'm':
			opts.useMBlocks = true
		case 'h':
			opts.humanReadable = true
		case 's':
			opts.summary = true
		case 'a':
			opts.allFiles = true
		case 'c':
			opts.grandTotal = true
		case 'b':
			opts.useBytes = true
			opts.apparentSize = true
		case 'd':
			return parseCombinedDepth(flags[i+1:], opts, rest)
		default:
			return false, 0
		}
	}
	return true, 0
}

// parseCombinedDepth handles -d within combined flags (e.g., -ad1).
func parseCombinedDepth(remainder string, opts *options, rest []string) (bool, int) {
	if remainder != "" {
		parseMaxDepthValue(remainder, opts)
		return true, 0
	}
	if len(rest) > 0 {
		parseMaxDepthValue(rest[0], opts)
		return true, 1
	}
	return true, 0
}

// processArg handles a single command-line argument and returns its total.
func (w *walker) processArg(path string) int64 {
	fi, err := sys.Lstat(path)
	if err != nil {
		w.reportError(path, err)
		return 0
	}
	if fi.Mode.IsDir() {
		return w.walkDir(path, fi, 0)
	}
	size := w.fileSize(fi)
	w.printEntry(size, path)
	return size
}

// walkDir reads a directory, recurses into children, and prints the
// accumulated size. R1.1, R1.3: format is "SIZE\tPATH\n".
func (w *walker) walkDir(path string, fi *sys.FileInfo, depth int) int64 {
	// Use block-based size for the directory entry itself, matching
	// GNU du behavior across filesystem types.
	total := fi.Blocks * 512
	entries, err := os.ReadDir(path)
	if err != nil {
		w.reportError(path, err)
		if w.shouldPrintDir(depth) {
			w.printEntry(total, path)
		}
		return total
	}
	for _, e := range entries {
		total += w.walkChild(joinPath(path, e.Name()), depth+1)
	}
	if w.shouldPrintDir(depth) {
		w.printEntry(total, path)
	}
	return total
}

// shouldPrintDir decides whether to print a directory at the given depth.
// R2.2: -s suppresses all except depth 0. R2.4: -d N limits to depth <= N.
func (w *walker) shouldPrintDir(depth int) bool {
	if w.opts.hasMaxDepth {
		return depth <= w.opts.maxDepth
	}
	return true
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
	size := w.fileSize(fi)
	if w.shouldPrintFile(depth) {
		w.printEntry(size, path)
	}
	return size
}

// shouldPrintFile decides whether to print a file at the given depth.
// R2.3: -a prints all files. R2.4: respects max depth.
func (w *walker) shouldPrintFile(depth int) bool {
	if !w.opts.allFiles {
		return false
	}
	if w.opts.hasMaxDepth {
		return depth <= w.opts.maxDepth
	}
	return true
}

// fileSize returns the size in bytes for a file, applying hard-link
// deduplication. R3.1: files with the same dev+ino counted once.
func (w *walker) fileSize(fi *sys.FileInfo) int64 {
	key := inode{Dev: fi.Dev, Ino: fi.Ino}
	if w.seen[key] {
		return 0
	}
	if fi.Nlink > 1 {
		w.seen[key] = true
	}
	return w.rawSize(fi)
}

// rawSize returns the size in bytes based on the measurement mode.
// R2.8: --apparent-size/-b uses st_size; default uses st_blocks * 512.
func (w *walker) rawSize(fi *sys.FileInfo) int64 {
	if w.opts.apparentSize {
		return fi.Size
	}
	return fi.Blocks * 512
}

// formatSize converts a size in bytes to the display unit.
// R2.1: -h uses HumanSize. R2.5: -k is 1K blocks (default).
// R2.6: -m is 1M blocks. -b is raw bytes.
func (w *walker) formatSize(sizeBytes int64) string {
	if w.opts.humanReadable {
		return format.HumanSize(sizeBytes, format.HumanSizeOpts{Binary: true})
	}
	if w.opts.useBytes {
		return fmt.Sprintf("%d", sizeBytes)
	}
	if w.opts.useMBlocks {
		return fmt.Sprintf("%d", ceilDiv(sizeBytes, 1048576))
	}
	// Default: 1024-byte (1K) blocks
	return fmt.Sprintf("%d", ceilDiv(sizeBytes, 1024))
}

// ceilDiv returns the ceiling of a / b for positive values.
func ceilDiv(a, b int64) int64 {
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

// printEntry prints one output line in "SIZE\tPATH\n" format (R1.3).
func (w *walker) printEntry(sizeBytes int64, path string) {
	fmt.Fprintf(os.Stdout, "%s\t%s\n", w.formatSize(sizeBytes), path)
}

// joinPath concatenates a parent directory and child name, preserving
// the parent path prefix to match GNU du behavior.
func joinPath(parent, child string) string {
	return parent + "/" + child
}

// reportError prints a diagnostic to stderr and sets the error flag (R4.2).
func (w *walker) reportError(path string, err error) {
	fmt.Fprintf(os.Stderr, "du: cannot access '%s': %s\n", path, osErrorMessage(err))
	w.hasError = true
}

// osErrorMessage extracts the underlying OS error message from a Go error.
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
