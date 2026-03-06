// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cmd/du binary.
// Reports disk usage for directories and files by recursively accumulating
// block counts, with support for summary mode, human-readable output,
// depth limiting, hard-link deduplication, and grand total reporting.
//
// Implements: prd009-du R1-R5
// Architecture: docs/ARCHITECTURE.yaml § cmd/
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// progName is the program name used in error diagnostics (R4.2).
const progName = "du"

// fileKey identifies a unique file by device and inode for hard-link
// deduplication across the entire invocation (R3.1, R3.2, R3.3).
type fileKey struct {
	Dev uint64
	Ino uint64
}

// sizeMode determines how file sizes are displayed.
type sizeMode int

const (
	modeKilo  sizeMode = iota // 1024-byte blocks (default, -k) (R1.2, R2.5)
	modeMega                  // 1048576-byte blocks (-m) (R2.6)
	modeHuman                 // human-readable K/M/G/T (-h) (R2.1)
	modeBytes                 // apparent size in bytes (-b)
)

// humanSuffixes lists binary (1024-based) unit suffixes for -h output (R2.1).
var humanSuffixes = [...]string{"", "K", "M", "G", "T", "P", "E"} //nolint:gochecknoglobals

func main() {
	// R5.1: SIGPIPE handler so piping to head exits cleanly.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGPIPE)
	go func() {
		<-sigCh
		os.Exit(0)
	}()

	var (
		flagAll          bool
		flagSummarize    bool
		flagTotal        bool
		flagMaxDepth     int
		flagHuman        bool
		flagKilo         bool
		flagMega         bool
		flagByteMode     bool
		flagApparentSize bool
	)

	// Define flags per prd009-du R2.
	flag.BoolVar(&flagAll, "a", false, "write counts for all files, not just directories")
	flag.BoolVar(&flagAll, "all", false, "write counts for all files, not just directories")
	flag.BoolVar(&flagSummarize, "s", false, "display only a total for each argument")
	flag.BoolVar(&flagSummarize, "summarize", false, "display only a total for each argument")
	flag.BoolVar(&flagTotal, "c", false, "produce a grand total")
	flag.BoolVar(&flagTotal, "total", false, "produce a grand total")
	flag.IntVar(&flagMaxDepth, "d", -1, "print total for directory depth N or fewer levels below the argument")
	flag.IntVar(&flagMaxDepth, "max-depth", -1, "like -d")
	flag.BoolVar(&flagHuman, "h", false, "print sizes in human readable format (K, M, G, T)")
	flag.BoolVar(&flagHuman, "human-readable", false, "like -h")
	flag.BoolVar(&flagKilo, "k", false, "like --block-size=1K (default)")
	flag.BoolVar(&flagMega, "m", false, "like --block-size=1M")
	flag.BoolVar(&flagByteMode, "b", false, "apparent size in bytes")
	flag.BoolVar(&flagByteMode, "bytes", false, "like -b")
	flag.BoolVar(&flagApparentSize, "apparent-size", false, "print apparent sizes rather than disk usage")
	flag.Parse()

	// R2.5: -k accepted for compatibility (1K blocks is already the default).
	_ = flagKilo

	// Resolve display mode. Priority: -b > -h > -m > -k.
	mode := modeKilo
	if flagMega {
		mode = modeMega
	}
	if flagHuman {
		mode = modeHuman
	}
	if flagByteMode {
		mode = modeBytes
		flagApparentSize = true
	}

	// R2.2: -s is equivalent to --max-depth=0.
	maxDepth := flagMaxDepth
	if flagSummarize {
		maxDepth = 0
	}

	// R1.1: default to current directory when no arguments are given.
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	// R3.3: deduplication map shared across all arguments.
	seen := make(map[fileKey]bool)
	exitCode := 0
	var grandTotal int64

	// R1.5: process arguments in command-line order.
	for _, arg := range args {
		total := processArg(arg, maxDepth, flagAll, flagApparentSize, mode, seen, &exitCode)
		grandTotal += total
	}

	// R2.7: grand total line.
	if flagTotal {
		fmt.Printf("%s\ttotal\n", formatSize(grandTotal, mode))
	}

	os.Exit(exitCode)
}

// processArg processes a single command-line argument (file or directory) and
// returns its total size in bytes.
func processArg(path string, maxDepth int, all bool, apparentSize bool, mode sizeMode, seen map[fileKey]bool, exitCode *int) int64 {
	fi, st, err := lstatFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %v\n", progName, path, sysErr(err))
		*exitCode = 1
		return 0
	}

	// Non-directory argument: print its size and return.
	if !fi.IsDir() {
		size := entryBytes(fi, st, apparentSize)
		key := fileKey{Dev: uint64(st.Dev), Ino: st.Ino}
		if seen[key] {
			size = 0
		} else {
			seen[key] = true
		}
		fmt.Printf("%s\t%s\n", formatSize(size, mode), path)
		return size
	}

	return walkDir(path, 0, maxDepth, all, apparentSize, mode, seen, exitCode)
}

// walkDir recursively processes a directory and returns its total size in bytes.
// Children are printed before the parent (bottom-up order, matching GNU du).
func walkDir(dirPath string, depth int, maxDepth int, all bool, apparentSize bool, mode sizeMode, seen map[fileKey]bool, exitCode *int) int64 {
	var total int64

	// R1.4: lstat the directory entry itself (do not follow symbolic links).
	dirFI, dirSt, err := lstatFile(dirPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %v\n", progName, dirPath, sysErr(err))
		*exitCode = 1
		return 0
	}
	dirKey := fileKey{Dev: uint64(dirSt.Dev), Ino: dirSt.Ino}
	if !seen[dirKey] {
		seen[dirKey] = true
		total += entryBytes(dirFI, dirSt, apparentSize)
	}

	// Read directory contents. Process whatever entries were read before any error.
	entries, readErr := os.ReadDir(dirPath)
	if readErr != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot read directory '%s': %v\n", progName, dirPath, sysErr(readErr))
		*exitCode = 1
	}

	for _, entry := range entries {
		childPath := joinPath(dirPath, entry.Name())

		// R1.4: entry.IsDir() returns false for symlinks, so symlinks are not followed.
		if entry.IsDir() {
			childTotal := walkDir(childPath, depth+1, maxDepth, all, apparentSize, mode, seen, exitCode)
			total += childTotal
		} else {
			childFI, childSt, err := lstatFile(childPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %v\n", progName, childPath, sysErr(err))
				*exitCode = 1
				continue
			}
			childKey := fileKey{Dev: uint64(childSt.Dev), Ino: childSt.Ino}
			childSize := int64(0)
			if !seen[childKey] {
				seen[childKey] = true
				childSize = entryBytes(childFI, childSt, apparentSize)
			}
			total += childSize

			// R2.3: with -a, print a line for every file.
			if all && canPrint(depth+1, maxDepth) {
				fmt.Printf("%s\t%s\n", formatSize(childSize, mode), childPath)
			}
		}
	}

	// Print directory total after children.
	if canPrint(depth, maxDepth) {
		fmt.Printf("%s\t%s\n", formatSize(total, mode), dirPath)
	}

	return total
}

// lstatFile returns os.FileInfo and the underlying syscall.Stat_t for path
// without following symbolic links (R1.4, D2).
func lstatFile(path string) (os.FileInfo, *syscall.Stat_t, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected Sys() type for %s", path)
	}
	return fi, st, nil
}

// entryBytes returns the size contribution of a file in bytes.
// When apparentSize is true, uses fi.Size() (st_size); otherwise uses
// st.Blocks * 512 to report physical disk allocation (R1.2, R2.8).
func entryBytes(fi os.FileInfo, st *syscall.Stat_t, apparentSize bool) int64 {
	if apparentSize {
		return fi.Size()
	}
	return st.Blocks * 512
}

// canPrint reports whether an entry at the given depth should be printed.
// A negative maxDepth means no limit (R2.4).
func canPrint(depth int, maxDepth int) bool {
	if maxDepth < 0 {
		return true
	}
	return depth <= maxDepth
}

// joinPath constructs a child path preserving the parent directory's prefix form.
// This ensures "./subdir" paths when the argument is "." (matching GNU du).
func joinPath(dir, name string) string {
	if len(dir) > 0 && dir[len(dir)-1] == '/' {
		return dir + name
	}
	return dir + "/" + name
}

// formatSize converts a byte count to the display string for the given mode.
func formatSize(bytes int64, mode sizeMode) string {
	switch mode {
	case modeKilo:
		return fmt.Sprintf("%d", ceilDiv(bytes, 1024))
	case modeMega:
		return fmt.Sprintf("%d", ceilDiv(bytes, 1048576))
	case modeHuman:
		return humanSize(bytes)
	case modeBytes:
		return fmt.Sprintf("%d", bytes)
	default:
		return fmt.Sprintf("%d", ceilDiv(bytes, 1024))
	}
}

// ceilDiv returns the ceiling of a/b for non-negative a and positive b.
// Returns 0 when a is zero or negative.
func ceilDiv(a, b int64) int64 {
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

// humanSize formats bytes as a human-readable string using binary (1024-based)
// units with suffixes K, M, G, T, P, E (R2.1).
func humanSize(bytes int64) string {
	if bytes == 0 {
		return "0"
	}
	const base = 1024.0
	value := float64(bytes)
	idx := 0
	for idx < len(humanSuffixes)-1 && value >= base {
		value /= base
		idx++
	}
	if idx == 0 {
		return fmt.Sprintf("%d", bytes)
	}
	// R2.1: one decimal place for all suffixed values.
	return fmt.Sprintf("%.1f%s", value, humanSuffixes[idx])
}

// sysErr extracts the underlying system error from an os.PathError for
// cleaner diagnostic messages matching GNU du format.
func sysErr(err error) error {
	if pe, ok := err.(*os.PathError); ok { //nolint:errorlint
		return pe.Err
	}
	return err
}
