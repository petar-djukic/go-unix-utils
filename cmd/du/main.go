// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the du utility for reporting disk usage.
//
// Implements prd009-du: recursive directory traversal (R1), flag behavior (R2),
// hard-link deduplication (R3), exit codes (R4), SIGPIPE handling (R5).
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// inodeKey is the deduplication key for hard-linked files. R3.2.
type inodeKey struct {
	Dev uint64
	Ino uint64
}

// blockUnit identifies the output unit mode.
type blockUnit int

const (
	unitKilo    blockUnit = iota // 1024-byte blocks (default, -k)
	unitMega                     // 1048576-byte blocks (-m)
	unitHuman                    // human-readable (-h)
)

// flags holds the parsed command-line options.
type flags struct {
	summarize    bool      // -s: print only total per argument
	allFiles     bool      // -a: print size for every file
	grandTotal   bool      // -c: print grand total after all arguments
	apparentSize bool      // --apparent-size: use st_size instead of st_blocks
	unit         blockUnit // output unit mode
	maxDepth     int       // -d N / --max-depth=N; -1 means unlimited
}

func main() {
	sys.InstallSIGPIPEHandler()

	f, paths := parseArgs(os.Args[1:])

	if len(paths) == 0 {
		paths = []string{"."}
	}

	exitCode := 0
	seen := make(map[inodeKey]bool)
	var grandTotal int64

	for _, p := range paths {
		total, err := duPath(p, f, seen)
		if err != nil {
			exitCode = 1
		}
		grandTotal += total
	}

	// R2.7: -c prints a grand total line.
	if f.grandTotal {
		printEntry(toDisplayUnits(grandTotal, f), "total", f)
	}

	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into flags and paths.
func parseArgs(args []string) (flags, []string) {
	var f flags
	f.maxDepth = -1 // unlimited by default
	var paths []string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endOfFlags {
			paths = append(paths, arg)
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		// Long flags.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--apparent-size":
				f.apparentSize = true
			case arg == "--summarize":
				f.summarize = true
			case arg == "--all":
				f.allFiles = true
			case arg == "--total":
				f.grandTotal = true
			case arg == "--human-readable":
				f.unit = unitHuman
			case strings.HasPrefix(arg, "--max-depth="):
				val := arg[len("--max-depth="):]
				n, err := strconv.Atoi(val)
				if err != nil || n < 0 {
					fmt.Fprintf(os.Stderr, "du: invalid maximum depth '%s'\n", val)
					os.Exit(1)
				}
				f.maxDepth = n
			default:
				fmt.Fprintf(os.Stderr, "du: unrecognized option '%s'\n", arg)
				os.Exit(1)
			}
			continue
		}

		// Short flags.
		if len(arg) > 1 && arg[0] == '-' {
			for j := 1; j < len(arg); j++ {
				ch := arg[j]
				switch ch {
				case 's':
					f.summarize = true
				case 'a':
					f.allFiles = true
				case 'c':
					f.grandTotal = true
				case 'h':
					f.unit = unitHuman
				case 'k':
					f.unit = unitKilo
				case 'm':
					f.unit = unitMega
				case 'd':
					// -d N: next argument or rest of current arg is the depth.
					rest := arg[j+1:]
					if rest != "" {
						n, err := strconv.Atoi(rest)
						if err != nil || n < 0 {
							fmt.Fprintf(os.Stderr, "du: invalid maximum depth '%s'\n", rest)
							os.Exit(1)
						}
						f.maxDepth = n
					} else {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "du: option requires an argument -- 'd'\n")
							os.Exit(1)
						}
						n, err := strconv.Atoi(args[i])
						if err != nil || n < 0 {
							fmt.Fprintf(os.Stderr, "du: invalid maximum depth '%s'\n", args[i])
							os.Exit(1)
						}
						f.maxDepth = n
					}
					j = len(arg) // consumed the value
				default:
					fmt.Fprintf(os.Stderr, "du: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			continue
		}

		paths = append(paths, arg)
	}

	// R2.2: -s is equivalent to --max-depth=0.
	if f.summarize && f.maxDepth == -1 {
		f.maxDepth = 0
	}

	return f, paths
}

// duPath traverses a single path argument and prints its disk usage.
// Returns the total raw size and any error.
func duPath(root string, f flags, seen map[inodeKey]bool) (int64, error) {
	// R1.4: Use Lstat to avoid following symbolic links.
	fi, err := sys.Lstat(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: cannot access '%s': No such file or directory\n", root)
		return 0, err
	}

	// If the root is a regular file (not a directory), just print it.
	if !fi.Mode.IsDir() {
		size := fileSize(fi, f, seen)
		printEntry(toDisplayUnits(size, f), root, f)
		return size, nil
	}

	total, hadError := walkDir(root, 0, f, seen)
	if hadError {
		return total, fmt.Errorf("errors during traversal")
	}
	return total, nil
}

// readDirUnsorted reads directory entries in filesystem order (not sorted).
// GNU du uses readdir order; Go's os.ReadDir sorts by name.
func readDirUnsorted(path string) ([]os.DirEntry, error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return dir.ReadDir(-1)
}

// walkDir recursively walks a directory and returns raw size (in 512-byte blocks
// or bytes for --apparent-size). It prints entries according to flag settings.
func walkDir(path string, depth int, f flags, seen map[inodeKey]bool) (int64, bool) {
	entries, err := readDirUnsorted(path)
	hadError := false
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: cannot read directory '%s': %v\n", path, err)
		hadError = true
	}

	var dirTotal int64

	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())

		// R1.4: Lstat — do not follow symlinks.
		fi, err := sys.Lstat(childPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "du: cannot access '%s': %v\n", childPath, err)
			hadError = true
			continue
		}

		if fi.Mode.IsDir() {
			subTotal, subErr := walkDir(childPath, depth+1, f, seen)
			dirTotal += subTotal
			if subErr {
				hadError = true
			}
		} else {
			size := fileSize(fi, f, seen)
			dirTotal += size

			// R2.3: -a prints a line for every file.
			if f.allFiles && shouldPrint(depth+1, f) {
				printEntry(toDisplayUnits(size, f), childPath, f)
			}
		}
	}

	// Add the directory entry's own disk usage (block allocation only).
	// GNU du does not count directory st_size in --apparent-size mode.
	fi, err := sys.Lstat(path)
	if err == nil {
		if f.apparentSize {
			// Track inode for dedup but don't add directory apparent size.
			key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
			seen[key] = true
		} else {
			dirTotal += fileSize(fi, f, seen)
		}
	}

	// Print this directory if within max-depth.
	if shouldPrint(depth, f) {
		printEntry(toDisplayUnits(dirTotal, f), path, f)
	}

	return dirTotal, hadError
}

// fileSize returns the raw size contribution of a file, applying hard-link dedup.
// Returns 512-byte blocks or bytes (for --apparent-size).
func fileSize(fi *sys.FileInfo, f flags, seen map[inodeKey]bool) int64 {
	// R3.1: Hard-link deduplication — skip files already counted.
	key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
	if seen[key] {
		return 0
	}
	seen[key] = true

	if f.apparentSize {
		// R2.8: Use st_size (apparent file size in bytes).
		return fi.Size
	}
	// Default: 512-byte blocks.
	return fi.Blocks
}

// toDisplayUnits converts raw size to the display value.
// Raw size is in 512-byte blocks (default) or bytes (--apparent-size).
// The returned value is what gets printed (in the selected unit).
func toDisplayUnits(raw int64, f flags) int64 {
	if f.unit == unitHuman {
		// For human-readable, return bytes so formatHuman can work.
		if f.apparentSize {
			return raw
		}
		return raw * 512
	}

	if f.unit == unitMega {
		// R2.6: Convert to 1M blocks, rounding up.
		var bytes int64
		if f.apparentSize {
			bytes = raw
		} else {
			bytes = raw * 512
		}
		return ceilDiv(bytes, 1048576)
	}

	// Default: 1K blocks. R1.2: Blocks / 2 (512-byte to 1K).
	if f.apparentSize {
		return ceilDiv(raw, 1024)
	}
	return ceilDiv(raw, 2)
}

// ceilDiv returns ceil(a/b) for positive a and b.
func ceilDiv(a, b int64) int64 {
	return (a + b - 1) / b
}

// shouldPrint returns true if an entry at the given depth should be printed.
func shouldPrint(depth int, f flags) bool {
	if f.maxDepth >= 0 && depth > f.maxDepth {
		return false
	}
	return true
}

// printEntry prints a single du output line.
func printEntry(size int64, path string, f flags) {
	if f.unit == unitHuman {
		fmt.Fprintf(os.Stdout, "%s\t%s\n", formatHuman(size), path)
	} else {
		fmt.Fprintf(os.Stdout, "%d\t%s\n", size, path)
	}
}

// humanUnit holds a threshold and suffix for human-readable formatting.
var humanUnits = []struct {
	thresh int64
	suffix string
}{
	{1 << 60, "E"},
	{1 << 50, "P"},
	{1 << 40, "T"},
	{1 << 30, "G"},
	{1 << 20, "M"},
	{1 << 10, "K"},
}

// formatHuman formats a byte count in GNU du -h style.
// Values < 10 with a suffix show one decimal place (e.g., "4.0K").
// Values >= 10 with a suffix show no decimal (e.g., "16K").
// Values without a suffix are plain integers.
func formatHuman(bytes int64) string {
	if bytes == 0 {
		return "0"
	}

	for _, u := range humanUnits {
		if bytes >= u.thresh {
			val := float64(bytes) / float64(u.thresh)
			if val < 10 {
				return fmt.Sprintf("%.1f%s", val, u.suffix)
			}
			return fmt.Sprintf("%.0f%s", math.Round(val), u.suffix)
		}
	}

	// Less than 1K, display as plain integer.
	return fmt.Sprintf("%d", bytes)
}
