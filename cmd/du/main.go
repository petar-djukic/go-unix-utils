// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd009-du R1.1–R1.5, R2.1–R2.8, R3.1–R3.3, R4.1–R4.2, R5.1.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "du"

// config holds parsed command-line options.
type config struct {
	summary      bool // R2.2: -s, print only total per argument
	human        bool // R2.1: -h, human-readable 1024-based output
	allFiles     bool // R2.3: -a, print entries for all files
	maxDepth     int  // R2.4: -d N / --max-depth=N, -1 = unlimited
	megaBlocks   bool // R2.6: -m, report in 1M blocks
	grandTotal   bool // R2.7: -c, print grand total
	apparentSize bool // R2.8: --apparent-size, use st_size instead of st_blocks
}

// inodeKey identifies a unique file by device and inode number.
// R3.2: used as the deduplication key for hard-link tracking.
type inodeKey struct {
	Dev uint64
	Ino uint64
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run processes path arguments and returns the exit code.
// R1.1: defaults to "." when no arguments are given.
// R1.5: processes multiple arguments in order.
// R2.7: prints grand total when -c is given.
// R3.3: inode tracking is shared across all arguments.
func run(args []string, stdout, stderr io.Writer) int {
	cfg, paths := parseFlags(args, stderr)
	if len(paths) == 0 {
		paths = []string{"."}
	}
	exitCode := 0
	var cumulative int64
	seen := make(map[inodeKey]bool) // R3.3: shared across arguments
	for _, p := range paths {
		total, err := duPath(p, cfg, seen, stdout, stderr)
		if err != nil {
			exitCode = 1
		}
		cumulative += total
	}
	if cfg.grandTotal {
		printEntry(stdout, cfg, cumulative, "total")
	}
	return exitCode
}

// parseFlags parses command-line flags and returns config and remaining paths.
func parseFlags(args []string, stderr io.Writer) (config, []string) {
	fs := flag.NewFlagSet(progName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var cfg config
	cfg.maxDepth = -1
	fs.BoolVar(&cfg.summary, "s", false, "display only a total for each argument")
	fs.BoolVar(&cfg.human, "h", false, "print sizes in human readable format")
	fs.BoolVar(&cfg.allFiles, "a", false, "write counts for all files")
	fs.IntVar(&cfg.maxDepth, "d", -1, "max depth of entries to display")
	fs.IntVar(&cfg.maxDepth, "max-depth", -1, "max depth of entries to display")
	// R2.5: -k is accepted for compatibility but has no visible effect.
	var kFlag bool
	fs.BoolVar(&kFlag, "k", false, "count in 1024-byte blocks (default)")
	fs.BoolVar(&cfg.megaBlocks, "m", false, "count in 1048576-byte blocks")
	fs.BoolVar(&cfg.grandTotal, "c", false, "produce a grand total")
	// R2.8: --apparent-size uses file size instead of block allocation.
	fs.BoolVar(&cfg.apparentSize, "apparent-size", false,
		"print apparent sizes rather than disk usage")
	// TODO: --exclude is listed in prd009-du non_goals (E6). Not implemented.
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	_ = kFlag // suppress unused warning
	// R2.2: -s is equivalent to --max-depth=0.
	if cfg.summary {
		cfg.maxDepth = 0
	}
	return cfg, fs.Args()
}

// shouldPrint returns true if an entry at the given depth should be printed.
// R2.4: entries deeper than maxDepth are accumulated but not printed.
func shouldPrint(cfg config, depth int) bool {
	return cfg.maxDepth < 0 || depth <= cfg.maxDepth
}

// fileBytes returns the size contribution of a file in bytes.
// R2.8: uses apparent size (st_size) when enabled, otherwise block allocation.
func fileBytes(fi *sys.FileInfo, cfg config) int64 {
	if cfg.apparentSize {
		return fi.Size
	}
	return fi.Blocks * 512
}

// trackInode checks whether a file has already been counted via hard-link
// deduplication. Returns 0 if the inode was already seen; otherwise records
// it and returns the file's byte count.
// R3.1: tracks files by dev+ino across the entire invocation.
func trackInode(fi *sys.FileInfo, cfg config, seen map[inodeKey]bool) int64 {
	key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
	if fi.Nlink > 1 && seen[key] {
		return 0
	}
	if fi.Nlink > 1 {
		seen[key] = true
	}
	return fileBytes(fi, cfg)
}

// duPath handles a single path argument. Returns total bytes.
// R1.4: uses Lstat to avoid following symbolic links.
// R4.2: prints diagnostic on error and continues.
func duPath(path string, cfg config, seen map[inodeKey]bool, stdout, stderr io.Writer) (int64, error) {
	fi, err := sys.Lstat(path)
	if err != nil {
		reportErr(stderr, path, err)
		return 0, err
	}
	if !fi.Mode.IsDir() {
		bytes := trackInode(fi, cfg, seen)
		printEntry(stdout, cfg, bytes, path)
		return bytes, nil
	}
	dirBytes := trackInode(fi, cfg, seen)
	return walkDir(path, dirBytes, 0, cfg, seen, stdout, stderr)
}

// walkDir recursively accumulates disk usage for a directory.
// Returns total in bytes. R1.1: prints each subdirectory after its
// children (depth-first order) when within maxDepth.
func walkDir(dirPath string, dirBytes int64, depth int, cfg config, seen map[inodeKey]bool, stdout, stderr io.Writer) (int64, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		reportReadDirErr(stderr, dirPath, err)
		if shouldPrint(cfg, depth) {
			printEntry(stdout, cfg, dirBytes, dirPath)
		}
		return dirBytes, err
	}
	childBytes, walkErr := processEntries(
		dirPath, entries, depth, cfg, seen, stdout, stderr)
	total := dirBytes + childBytes
	if shouldPrint(cfg, depth) {
		printEntry(stdout, cfg, total, dirPath)
	}
	return total, walkErr
}

// processEntries iterates directory entries and accumulates byte counts.
func processEntries(dirPath string, entries []os.DirEntry, depth int, cfg config, seen map[inodeKey]bool, stdout, stderr io.Writer) (int64, error) {
	var total int64
	var hadErr error
	for _, entry := range entries {
		b, err := processOneEntry(
			dirPath, entry, depth, cfg, seen, stdout, stderr)
		if err != nil {
			hadErr = err
		}
		total += b
	}
	return total, hadErr
}

// processOneEntry handles a single directory entry, returning its byte count.
// R2.3: prints file entries when allFiles is enabled and within maxDepth.
// R3.1: deduplicates hard-linked files via inode tracking.
func processOneEntry(dirPath string, entry os.DirEntry, parentDepth int, cfg config, seen map[inodeKey]bool, stdout, stderr io.Writer) (int64, error) {
	entryPath := joinPath(dirPath, entry.Name())
	fi, err := sys.Lstat(entryPath) // R1.4: do not follow symlinks
	if err != nil {
		reportErr(stderr, entryPath, err)
		return 0, err
	}
	childDepth := parentDepth + 1
	if fi.Mode.IsDir() {
		dirBytes := trackInode(fi, cfg, seen)
		return walkDir(entryPath, dirBytes, childDepth, cfg, seen,
			stdout, stderr)
	}
	b := trackInode(fi, cfg, seen)
	if cfg.allFiles && shouldPrint(cfg, childDepth) {
		printEntry(stdout, cfg, b, entryPath)
	}
	return b, nil
}

// humanSuffixes for binary (1024-based) mode, matching GNU du -h.
var humanSuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// formatHuman formats bytes as a human-readable string matching GNU du -h.
// Uses 1024-based units with one decimal place for values with a suffix.
func formatHuman(bytes int64) string {
	if bytes == 0 {
		return "0"
	}
	value := float64(bytes)
	for i := 0; i < len(humanSuffixes)-1; i++ {
		if value < 1024.0 {
			if humanSuffixes[i] == "" {
				return fmt.Sprintf("%.0f", value)
			}
			return fmt.Sprintf("%.1f%s", value, humanSuffixes[i])
		}
		value /= 1024.0
	}
	return fmt.Sprintf("%.1f%s", value, humanSuffixes[len(humanSuffixes)-1])
}

// printEntry outputs one du line. R1.3: format is "SIZE\tPATH\n".
// R2.1: uses human-readable format when enabled.
// R2.6: uses 1M blocks when megaBlocks is enabled.
// Internal representation is bytes; converted to display units here.
func printEntry(w io.Writer, cfg config, bytes int64, path string) {
	if cfg.human {
		fmt.Fprintf(w, "%s\t%s\n", formatHuman(bytes), path)
		return
	}
	if cfg.megaBlocks {
		fmt.Fprintf(w, "%d\t%s\n", ceilDiv(bytes, 1048576), path)
		return
	}
	// Default: 1K blocks.
	fmt.Fprintf(w, "%d\t%s\n", bytes/1024, path)
}

// ceilDiv returns the ceiling division of a by b.
func ceilDiv(a, b int64) int64 {
	return (a + b - 1) / b
}

// joinPath joins a directory and entry name without cleaning the path,
// preserving prefixes like "./" that filepath.Join would remove.
func joinPath(dir, name string) string {
	return dir + string(os.PathSeparator) + name
}

// reportErr writes a diagnostic to stderr for a path error.
// R4.2: prints diagnostic and continues processing.
func reportErr(w io.Writer, path string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(w, "%s: cannot access '%s': %s\n",
			progName, path, pathErr.Err)
		return
	}
	fmt.Fprintf(w, "%s: %s: %s\n", progName, path, err)
}

// reportReadDirErr writes a diagnostic for directory read errors.
// R4.2: uses "cannot read directory" to match GNU du format.
func reportReadDirErr(w io.Writer, path string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(w, "%s: cannot read directory '%s': %s\n",
			progName, path, pathErr.Err)
		return
	}
	fmt.Fprintf(w, "%s: %s: %s\n", progName, path, err)
}
