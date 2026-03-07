// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/du: report disk usage for files and directories.
// Implements prd009-du R1-R5.
package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// exitCode tracks whether any error occurred during processing.
// R4.1, R4.2: exit 0 on success, exit 1 on any error.
var exitCode int

// duFlags holds the parsed command-line flags for du.
type duFlags struct {
	humanReadable bool // -h: human-readable sizes (R2.1)
	summary       bool // -s: summarize (R2.2)
	allFiles      bool // -a: all files (R2.3)
	grandTotal    bool // -c: grand total (R2.7)
	megabytes     bool // -m: 1M blocks (R2.6)
	maxDepth      int  // -d N: depth limit, -1 = unlimited (R2.4)
	apparentSize  bool // --apparent-size (R2.8)
}

// inodeKey identifies a unique file by device and inode for hard-link dedup.
// R3.1, R3.2: track files by st_dev and st_ino.
type inodeKey struct {
	Dev uint64
	Ino uint64
}

func main() {
	// D2, R5.1: Install SIGPIPE handler per ARCHITECTURE.yaml shared_protocols.
	sys.InstallSIGPIPEHandler()

	fl, args := parseArgs(os.Args[1:])

	if len(args) == 0 {
		// R1.1: default to current directory.
		args = []string{"."}
	}

	// R3.3: dedup map spans entire invocation, not per argument.
	seen := make(map[inodeKey]bool)
	var grandTotal int64

	// R1.5: process arguments in order.
	for _, arg := range args {
		total := processArg(arg, fl, seen)
		grandTotal += total
	}

	// R2.7: grand total line.
	if fl.grandTotal {
		printEntry(grandTotal, "total", fl)
	}

	os.Exit(exitCode)
}

// parseArgs parses du flags from args, supporting combined short flags, long
// options, and --.
func parseArgs(args []string) (*duFlags, []string) {
	fl := &duFlags{maxDepth: -1}
	var paths []string
	endFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endFlags || arg == "" || (arg[0] != '-') {
			paths = append(paths, arg)
			continue
		}

		if arg == "--" {
			endFlags = true
			continue
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--human-readable":
				fl.humanReadable = true
			case arg == "--summarize":
				fl.summary = true
				fl.maxDepth = 0
			case arg == "--total":
				fl.grandTotal = true
			case arg == "--all":
				fl.allFiles = true
			case arg == "--apparent-size":
				fl.apparentSize = true
			case strings.HasPrefix(arg, "--max-depth="):
				val := arg[len("--max-depth="):]
				n, err := strconv.Atoi(val)
				if err != nil {
					fmt.Fprintf(os.Stderr, "du: invalid maximum depth %q\n", val)
					os.Exit(1)
				}
				fl.maxDepth = n
			default:
				fmt.Fprintf(os.Stderr, "du: unrecognized option %q\n", arg)
				os.Exit(1)
			}
			continue
		}

		// Short options: parse combined flags like -shk.
		j := 1
		for j < len(arg) {
			ch := arg[j]
			switch ch {
			case 'h':
				fl.humanReadable = true
			case 's':
				fl.summary = true
				fl.maxDepth = 0
			case 'c':
				fl.grandTotal = true
			case 'a':
				fl.allFiles = true
			case 'k':
				// R2.5: 1K blocks is already the default; accepted for compat.
			case 'm':
				fl.megabytes = true
			case 'd':
				// -d takes the next characters or next argument as depth value.
				rest := arg[j+1:]
				if rest != "" {
					n, err := strconv.Atoi(rest)
					if err != nil {
						fmt.Fprintf(os.Stderr, "du: invalid maximum depth %q\n", rest)
						os.Exit(1)
					}
					fl.maxDepth = n
				} else {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "du: option requires an argument -- 'd'\n")
						os.Exit(1)
					}
					n, err := strconv.Atoi(args[i])
					if err != nil {
						fmt.Fprintf(os.Stderr, "du: invalid maximum depth %q\n", args[i])
						os.Exit(1)
					}
					fl.maxDepth = n
				}
				j = len(arg) // consumed rest of this arg
				continue
			default:
				fmt.Fprintf(os.Stderr, "du: invalid option -- '%c'\n", ch)
				os.Exit(1)
			}
			j++
		}
	}

	return fl, paths
}

// processArg handles a single command-line argument (file or directory).
func processArg(path string, fl *duFlags, seen map[inodeKey]bool) int64 {
	fi, err := sys.Lstat(path)
	if err != nil {
		reportError("cannot access", path, err)
		return 0
	}

	if !fi.Mode.IsDir() {
		size := entrySize(fi, fl, seen)
		printEntry(size, path, fl)
		return size
	}

	return walkDir(path, fl, seen, 0)
}

// walkDir recursively traverses a directory, accumulating sizes and printing
// entries according to flags. Returns the total size in bytes for the directory.
func walkDir(dirPath string, fl *duFlags, seen map[inodeKey]bool, depth int) int64 {
	var total int64

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		reportError("cannot read directory", dirPath, err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())

		if entry.IsDir() {
			subtotal := walkDir(entryPath, fl, seen, depth+1)
			total += subtotal
			continue
		}

		fi, err := sys.Lstat(entryPath)
		if err != nil {
			reportError("cannot access", entryPath, err)
			continue
		}

		size := entrySize(fi, fl, seen)
		total += size

		// R2.3: with -a, print individual files if within depth.
		if fl.allFiles && depthInRange(depth+1, fl) {
			printEntry(size, entryPath, fl)
		}
	}

	// Add the directory entry's own disk usage.
	fi, err := sys.Lstat(dirPath)
	if err == nil {
		total += entrySize(fi, fl, seen)
	}

	// Print directory if within depth.
	if depthInRange(depth, fl) {
		printEntry(total, dirPath, fl)
	}

	return total
}

// entrySize returns the size in bytes for a file entry, applying hard-link dedup
// and apparent-size mode.
func entrySize(fi *sys.FileInfo, fl *duFlags, seen map[inodeKey]bool) int64 {
	// R3.1, R3.2: hard-link deduplication for non-directory entries.
	if !fi.Mode.IsDir() && fi.Nlink > 1 {
		key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
		if seen[key] {
			return 0
		}
		seen[key] = true
	}

	// R2.8: --apparent-size uses st_size instead of st_blocks.
	if fl.apparentSize {
		return fi.Size
	}
	// R1.2: disk usage in bytes from 512-byte blocks.
	return fi.Blocks * 512
}

// depthInRange returns true if the given depth should be printed.
func depthInRange(depth int, fl *duFlags) bool {
	return fl.maxDepth < 0 || depth <= fl.maxDepth
}

// printEntry formats and prints a single du output line.
// R1.3: format is "SIZE\tPATH\n".
func printEntry(sizeBytes int64, path string, fl *duFlags) {
	var sizeStr string
	switch {
	case fl.humanReadable:
		// R2.1: human-readable binary (1024-based) format matching GNU du.
		sizeStr = formatHumanReadable(sizeBytes)
	case fl.megabytes:
		// R2.6: 1M blocks, rounding up.
		sizeStr = strconv.FormatInt(ceilDiv(sizeBytes, 1048576), 10)
	default:
		// R1.2, R2.5: 1K blocks.
		sizeStr = strconv.FormatInt(ceilDiv(sizeBytes, 1024), 10)
	}
	fmt.Printf("%s\t%s\n", sizeStr, path)
}

// ceilDiv returns the ceiling of a / b for positive values.
func ceilDiv(a, b int64) int64 {
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

// formatHumanReadable formats bytes as a human-readable string with base-1024
// units and suffixes K/M/G/T/P/E, matching GNU du -h output format.
// Values >= 10 in the current unit: no decimal. Values < 10: one decimal.
func formatHumanReadable(bytes int64) string {
	if bytes == 0 {
		return "0"
	}

	suffixes := [...]string{"", "K", "M", "G", "T", "P", "E"}
	val := float64(bytes)

	for i, suffix := range suffixes {
		if i == len(suffixes)-1 || val < 1024 {
			if suffix == "" {
				return strconv.FormatInt(int64(math.Ceil(val)), 10)
			}
			if val >= 10 {
				return fmt.Sprintf("%.0f%s", math.Ceil(val), suffix)
			}
			// One decimal place for values < 10.
			rounded := math.Ceil(val*10) / 10
			return fmt.Sprintf("%.1f%s", rounded, suffix)
		}
		val /= 1024
	}

	last := suffixes[len(suffixes)-1]
	return fmt.Sprintf("%.1f%s", val, last)
}

// reportError prints a diagnostic to stderr and sets exit code to 1.
// R4.2: print error, continue processing.
func reportError(action, path string, err error) {
	exitCode = 1
	// Extract the underlying OS error message for GNU-compatible formatting.
	var pe *os.PathError
	var msg string
	if errors.As(err, &pe) {
		msg = capitalizeFirst(pe.Err.Error())
	} else {
		msg = capitalizeFirst(err.Error())
	}
	fmt.Fprintf(os.Stderr, "du: %s '%s': %s\n", action, path, msg)
}

// capitalizeFirst returns s with its first rune uppercased.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
