// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd009-du R1.1-R1.5, R2.1-R2.7, R3.1-R3.3, R4.1-R4.2, R5.1:
// cmd/du reports disk usage for directory trees. Walks each path argument
// recursively using Lstat (no symlink following), summing allocated 512-byte
// blocks with hard-link deduplication. Prints one line per directory
// (or per file with -a) in depth-first order. Installs SIGPIPE handler
// per ARCHITECTURE.yaml.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU du format.
const progName = "du"

// inodeKey identifies a unique file by device and inode for hard-link
// deduplication per R3.1-R3.2.
type inodeKey struct {
	Dev uint64
	Ino uint64
}

// duOptions holds the parsed flags for a du invocation.
type duOptions struct {
	summarize  bool  // -s/--summarize: display only total per argument (R2.2)
	all        bool  // -a/--all: report sizes for all files (R2.3)
	blockSize  int64 // display block size in bytes (R1.2)
	maxDepth   int   // -d N/--max-depth=N: limit depth of reported entries (R2.4), -1 means unlimited
	grandTotal bool  // -c/--total: print grand total after all arguments (R2.7)
}

func main() {
	// R5.1: Install SIGPIPE handler for clean pipe exit.
	sys.InstallSIGPIPEHandler()

	opts, paths, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		os.Exit(1)
	}

	// R1.1: Default to current directory when no arguments given.
	if len(paths) == 0 {
		paths = []string{"."}
	}

	exitCode := 0
	// R3.3: Hard-link deduplication map shared across all arguments.
	seen := make(map[inodeKey]bool)

	var grandTotal int64

	// R1.5: Process arguments in order.
	for _, path := range paths {
		total := walkPath(path, opts, seen, &exitCode)
		if opts.summarize {
			printEntry(toDisplayBlocks(total, opts.blockSize), path)
		}
		grandTotal += total
	}

	// R2.7: -c prints a grand total line after all arguments.
	if opts.grandTotal {
		printEntry(toDisplayBlocks(grandTotal, opts.blockSize), "total")
	}

	os.Exit(exitCode)
}

// parseArgs separates flags from path arguments.
func parseArgs(args []string) (*duOptions, []string, error) {
	opts := &duOptions{
		blockSize: defaultBlockSize(),
		maxDepth:  -1, // R2.4: -1 means unlimited depth.
	}
	var paths []string
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone {
			paths = append(paths, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "--summarize" {
			opts.summarize = true
			continue
		}
		if arg == "--all" {
			opts.all = true
			continue
		}
		if arg == "--total" {
			// R2.7: --total is the long form of -c.
			opts.grandTotal = true
			continue
		}
		// R2.4: --max-depth=N long form.
		if strings.HasPrefix(arg, "--max-depth=") {
			val := arg[len("--max-depth="):]
			d, err := strconv.Atoi(val)
			if err != nil || d < 0 {
				return nil, nil, fmt.Errorf("invalid maximum depth '%s'", val)
			}
			opts.maxDepth = d
			continue
		}
		if arg == "--max-depth" {
			i++
			if i >= len(args) {
				return nil, nil, fmt.Errorf("option '--max-depth' requires an argument")
			}
			d, err := strconv.Atoi(args[i])
			if err != nil || d < 0 {
				return nil, nil, fmt.Errorf("invalid maximum depth '%s'", args[i])
			}
			opts.maxDepth = d
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && !strings.HasPrefix(arg, "--") {
			// R2.4: -d may be followed by the depth value in the same or next arg.
			chars := arg[1:]
			for j := 0; j < len(chars); j++ {
				ch := chars[j]
				switch ch {
				case 's':
					opts.summarize = true
				case 'a':
					opts.all = true
				case 'c':
					// R2.7: -c prints grand total.
					opts.grandTotal = true
				case 'k':
					// R2.5: -k forces 1024-byte blocks (already default).
					opts.blockSize = 1024
				case 'm':
					// R2.6: -m forces 1048576-byte (1M) blocks.
					opts.blockSize = 1048576
				case 'd':
					// R2.4: -d N depth limit. Value may be rest of this arg or next arg.
					rest := chars[j+1:]
					var val string
					if len(rest) > 0 {
						val = string(rest)
					} else {
						i++
						if i >= len(args) {
							return nil, nil, fmt.Errorf("option requires an argument -- 'd'")
						}
						val = args[i]
					}
					d, err := strconv.Atoi(val)
					if err != nil || d < 0 {
						return nil, nil, fmt.Errorf("invalid maximum depth '%s'", val)
					}
					opts.maxDepth = d
					j = len(chars) // consume rest of short flag group
				}
			}
			continue
		}
		paths = append(paths, arg)
	}

	// R2.4: -s is equivalent to --max-depth=0.
	if opts.summarize && opts.maxDepth == -1 {
		opts.maxDepth = 0
	}

	// R1.3: -s and -a are mutually exclusive.
	if opts.summarize && opts.all {
		return nil, nil, fmt.Errorf("cannot both summarize and show all entries")
	}

	return opts, paths, nil
}

// defaultBlockSize determines the display block size from environment
// variables, matching GNU du behavior per R1.2.
func defaultBlockSize() int64 {
	// R1.2: Check environment variables in precedence order.
	for _, envVar := range []string{"DU_BLOCK_SIZE", "BLOCK_SIZE", "BLOCKSIZE"} {
		if val, ok := os.LookupEnv(envVar); ok {
			if bs, err := strconv.ParseInt(val, 10, 64); err == nil && bs > 0 {
				return bs
			}
		}
	}
	// R1.2: POSIXLY_CORRECT forces 512-byte blocks.
	if _, ok := os.LookupEnv("POSIXLY_CORRECT"); ok {
		return 512
	}
	// R1.2: GNU du default is 1024-byte blocks.
	return 1024
}

// toDisplayBlocks converts raw 512-byte block counts to display block units
// using ceiling division.
func toDisplayBlocks(raw512Blocks int64, blockSize int64) int64 {
	if blockSize == 512 {
		return raw512Blocks
	}
	rawBytes := raw512Blocks * 512
	return (rawBytes + blockSize - 1) / blockSize
}

// printEntry outputs one du line in "SIZE\tPATH\n" format per R1.3.
func printEntry(displayBlocks int64, path string) {
	fmt.Printf("%d\t%s\n", displayBlocks, path)
}

// walkPath processes a single command-line argument and returns the total
// raw 512-byte block count. Errors are reported to stderr per R4.2.
func walkPath(path string, opts *duOptions, seen map[inodeKey]bool, exitCode *int) int64 {
	// R1.4: Use Lstat to avoid following symlinks.
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %v\n", progName, path, unwrapPathError(err))
		*exitCode = 1
		return 0
	}

	if !fi.Mode.IsDir() {
		blocks, _ := countBlocks(fi, seen)
		if !opts.summarize {
			printEntry(toDisplayBlocks(blocks, opts.blockSize), path)
		}
		return blocks
	}

	// R2.4: depth 0 is the argument itself.
	return walkDir(path, fi, opts, seen, exitCode, 0)
}

// shouldPrint returns true if the entry at the given depth should be printed,
// considering summarize and maxDepth settings per R2.2 and R2.4.
func shouldPrint(opts *duOptions, depth int) bool {
	if opts.summarize {
		return false
	}
	if opts.maxDepth >= 0 && depth > opts.maxDepth {
		return false
	}
	return true
}

// walkDir recursively walks a directory and returns the total raw 512-byte
// block count. Prints entries in depth-first order per R1.1.
// R2.4: depth tracks the current depth relative to the command-line argument (0 = the argument itself).
func walkDir(dirPath string, dirFI *sys.FileInfo, opts *duOptions, seen map[inodeKey]bool, exitCode *int, depth int) int64 {
	total, _ := countBlocks(dirFI, seen)

	// Use os.Open + ReadDir(-1) instead of os.ReadDir to preserve filesystem
	// readdir() order, matching GNU du's FTS traversal order.
	dirFile, err := os.Open(dirPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot read directory '%s': %v\n", progName, dirPath, unwrapPathError(err))
		*exitCode = 1
		if shouldPrint(opts, depth) {
			printEntry(toDisplayBlocks(total, opts.blockSize), dirPath)
		}
		return total
	}
	entries, err := dirFile.ReadDir(-1)
	dirFile.Close() // best-effort close after reading entries
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot read directory '%s': %v\n", progName, dirPath, unwrapPathError(err))
		*exitCode = 1
		if shouldPrint(opts, depth) {
			printEntry(toDisplayBlocks(total, opts.blockSize), dirPath)
		}
		return total
	}

	for _, entry := range entries {
		childPath := dirPath + "/" + entry.Name()

		// R1.4: Use Lstat to avoid following symlinks.
		childFI, err := sys.Lstat(childPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %v\n", progName, childPath, unwrapPathError(err))
			*exitCode = 1
			continue
		}

		if childFI.Mode.IsDir() {
			total += walkDir(childPath, childFI, opts, seen, exitCode, depth+1)
		} else {
			blocks, dup := countBlocks(childFI, seen)
			total += blocks
			// R2.3: -a prints entries for all files, not just directories.
			// Hard-link duplicates are not printed (GNU du behavior).
			if opts.all && !dup && shouldPrint(opts, depth+1) {
				printEntry(toDisplayBlocks(blocks, opts.blockSize), childPath)
			}
		}
	}

	// R1.1: Print directory entry after children (depth-first order).
	if shouldPrint(opts, depth) {
		printEntry(toDisplayBlocks(total, opts.blockSize), dirPath)
	}

	return total
}

// countBlocks returns the raw 512-byte block count for a file, applying
// hard-link deduplication per R3.1-R3.2. The second return value is true
// when the inode was already counted (duplicate hard link).
func countBlocks(fi *sys.FileInfo, seen map[inodeKey]bool) (int64, bool) {
	key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
	// R3.1: Track files with nlink > 1 to prevent double-counting hard links.
	if fi.Nlink > 1 {
		if seen[key] {
			return 0, true
		}
		seen[key] = true
	}
	return fi.Blocks, false
}

// unwrapPathError extracts the inner error from *os.PathError for cleaner
// error messages matching GNU du format.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
