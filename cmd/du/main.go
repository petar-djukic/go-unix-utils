// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the du utility for reporting disk usage.
// Implements prd009-du (R1, R2, R3, R4, R5).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// duFlags holds parsed command-line flags for du.
type duFlags struct {
	summary      bool // -s: print only total per argument
	grandTotal   bool // -c: print grand total
	allFiles     bool // -a: print all files, not just directories
	maxDepth     int  // -d N / --max-depth=N: limit depth (-1 means unlimited)
	humanRead    bool // -h: human-readable output (1024-based)
	megaBlocks   bool // -m: 1M blocks
	apparentSize bool // --apparent-size: use st_size instead of st_blocks
}

// inodeKey identifies a file by device and inode for hard-link deduplication.
// R3.2: keyed by struct{ Dev, Ino uint64 } pair.
type inodeKey struct {
	Dev uint64
	Ino uint64
}

func main() {
	// R5.1: install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	fl, files := parseArgs(os.Args[1:])
	if len(files) == 0 {
		files = []string{"."}
	}

	exitCode := 0
	seen := make(map[inodeKey]bool)
	var grandTotal int64

	w := bufio.NewWriter(os.Stdout)

	// R1.5: process multiple arguments in order.
	for _, path := range files {
		total, err := duWalk(w, path, &fl, seen, 0)
		if err != nil {
			exitCode = 1
		}
		grandTotal += total
	}

	// R2.7: grand total line.
	if fl.grandTotal {
		printLine(w, grandTotal, "total", &fl)
	}

	w.Flush() // flush before exit; os.Exit does not run defers
	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into flags and file paths.
func parseArgs(args []string) (duFlags, []string) {
	var fl duFlags
	fl.maxDepth = -1 // unlimited
	var files []string
	endOfFlags := false

	i := 0
	for i < len(args) {
		arg := args[i]

		if endOfFlags {
			files = append(files, arg)
			i++
			continue
		}

		if arg == "--" {
			endOfFlags = true
			i++
			continue
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--apparent-size":
				fl.apparentSize = true
			case arg == "--max-depth":
				i++
				if i >= len(args) {
					fmt.Fprintln(os.Stderr, "du: option '--max-depth' requires an argument")
					os.Exit(1)
				}
				fl.maxDepth = parseDepth(args[i])
			case strings.HasPrefix(arg, "--max-depth="):
				fl.maxDepth = parseDepth(arg[len("--max-depth="):])
			default:
				fmt.Fprintf(os.Stderr, "du: unrecognized option '%s'\n", arg)
				os.Exit(1)
			}
			i++
			continue
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' {
			j := 1
			for j < len(arg) {
				switch arg[j] {
				case 's':
					fl.summary = true
				case 'c':
					fl.grandTotal = true
				case 'a':
					fl.allFiles = true
				case 'h':
					fl.humanRead = true
				case 'k':
					// R2.5: accepted, no visible effect (1K is default).
				case 'm':
					fl.megaBlocks = true
				case 'd':
					rest := arg[j+1:]
					if rest != "" {
						fl.maxDepth = parseDepth(rest)
					} else {
						i++
						if i >= len(args) {
							fmt.Fprintln(os.Stderr, "du: option requires an argument -- 'd'")
							os.Exit(1)
						}
						fl.maxDepth = parseDepth(args[i])
					}
					j = len(arg) // consumed remaining chars
					continue
				default:
					fmt.Fprintf(os.Stderr, "du: invalid option -- '%c'\n", arg[j])
					os.Exit(1)
				}
				j++
			}
			i++
			continue
		}

		files = append(files, arg)
		i++
	}

	// R2.2: -s is equivalent to --max-depth=0.
	if fl.summary {
		fl.maxDepth = 0
	}

	return fl, files
}

// parseDepth parses a max-depth value and exits on invalid input.
func parseDepth(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		fmt.Fprintf(os.Stderr, "du: invalid maximum depth '%s'\n", s)
		os.Exit(1)
	}
	return n
}

// fileBytes returns the byte count for a regular file based on the apparent-size flag.
// R2.8: --apparent-size uses st_size; default uses st_blocks * 512.
func fileBytes(fi *sys.FileInfo, fl *duFlags) int64 {
	if fl.apparentSize {
		return fi.Size
	}
	return fi.Blocks * 512
}

// dirBytes returns the byte count for a directory entry itself.
// Directories use block-based accounting (st_blocks * 512) regardless of
// --apparent-size, matching GNU du behavior on filesystems where directories
// have st_blocks=0 (e.g., APFS).
func dirBytes(fi *sys.FileInfo) int64 {
	return fi.Blocks * 512
}

// duWalk recursively traverses path, printing entries and returning total bytes.
// R1.1: recurses into directories. R3.1: deduplicates hard links via seen map.
func duWalk(w *bufio.Writer, path string, fl *duFlags, seen map[inodeKey]bool, depth int) (int64, error) {
	// R1.4: use Lstat to avoid following symbolic links.
	fi, err := sys.Lstat(path)
	if err != nil {
		// R4.2: print diagnostic to stderr, continue processing.
		fmt.Fprintf(os.Stderr, "du: cannot access '%s': %s\n", path, extractError(err))
		return 0, err
	}

	// Non-directory entry (regular file, symlink, etc.).
	if !fi.Mode.IsDir() {
		// R3.1: check hard-link dedup.
		key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
		if seen[key] {
			return 0, nil
		}
		seen[key] = true

		b := fileBytes(fi, fl)
		// Top-level arguments always print; deeper files only with -a.
		if depth == 0 || (fl.allFiles && (fl.maxDepth < 0 || depth <= fl.maxDepth)) {
			printLine(w, b, path, fl)
		}
		return b, nil
	}

	// Directory: read entries and recurse.
	// Use File.ReadDir to preserve filesystem order, matching GNU du (fts).
	entries, readErr := readDir(path)
	var hadError bool
	if readErr != nil {
		fmt.Fprintf(os.Stderr, "du: cannot read directory '%s': %s\n", path, extractError(readErr))
		hadError = true
	}

	var totalBytes int64
	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())
		childBytes, childErr := duWalk(w, childPath, fl, seen, depth+1)
		totalBytes += childBytes
		if childErr != nil {
			hadError = true
		}
	}

	// Add directory's own allocation.
	totalBytes += dirBytes(fi)

	// Print directory line if within depth limit.
	if fl.maxDepth < 0 || depth <= fl.maxDepth {
		printLine(w, totalBytes, path, fl)
	}

	if hadError {
		return totalBytes, fmt.Errorf("errors during traversal of %s", path)
	}
	return totalBytes, nil
}

// printLine writes one "SIZE\tPATH\n" output line.
// R1.3: tab-separated, no leading spaces on SIZE.
func printLine(w *bufio.Writer, bytes int64, path string, fl *duFlags) {
	fmt.Fprintf(w, "%s\t%s\n", formatSize(bytes, fl), path) //nolint:errcheck
}

// formatSize converts bytes to the display string based on flags.
func formatSize(bytes int64, fl *duFlags) string {
	if fl.humanRead {
		// R2.1: human-readable, binary mode (1024-based).
		return formatHumanSize(bytes)
	}
	if fl.megaBlocks {
		// R2.6: 1M blocks, rounding up.
		return strconv.FormatInt(ceilDiv(bytes, 1048576), 10)
	}
	// R1.2: default 1K blocks.
	return strconv.FormatInt(ceilDiv(bytes, 1024), 10)
}

// formatHumanSize formats bytes as a human-readable string with binary (1024-based)
// unit suffixes: K, M, G, T, P, E. Matches GNU coreutils human_readable output.
// R2.1: binary mode, powers of 1024.
func formatHumanSize(bytes int64) string {
	if bytes == 0 {
		return "0"
	}

	units := []string{"", "K", "M", "G", "T", "P", "E"}
	val := float64(bytes)
	unitIdx := 0

	for unitIdx < len(units)-1 && math.Abs(val) >= 1024 {
		val /= 1024
		unitIdx++
	}

	if unitIdx == 0 {
		// Plain bytes, no suffix.
		return strconv.FormatInt(bytes, 10)
	}

	if val < 10 {
		// One decimal place, ceiling-rounded.
		rounded := math.Ceil(val*10) / 10
		return fmt.Sprintf("%.1f%s", rounded, units[unitIdx])
	}
	// No decimal, ceiling-rounded.
	rounded := int64(math.Ceil(val))
	return fmt.Sprintf("%d%s", rounded, units[unitIdx])
}

// ceilDiv returns the ceiling of a / b for positive values.
func ceilDiv(a, b int64) int64 {
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

// readDir reads directory entries in filesystem order (not sorted).
// GNU du uses fts which processes entries in readdir order; os.ReadDir sorts
// alphabetically, causing output order mismatches. (*os.File).ReadDir preserves
// the filesystem's native ordering.
func readDir(path string) ([]os.DirEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() // best-effort close
	entries, err := f.ReadDir(-1)
	return entries, err
}

// extractError extracts the innermost error message for clean diagnostics.
func extractError(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}
