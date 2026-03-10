// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd009-du R1.1–R1.5 (directory traversal, size accumulation,
// output format, symlink handling, multiple arguments),
// R2.1 (-h human-readable), R2.2 (-s summary), R2.3 (-a all files),
// R2.4 (-d N/--max-depth=N depth limiting), R2.5 (-k), R2.6 (-m),
// R2.7 (-c grand total), R2.8 (--apparent-size),
// R3.1–R3.3 (hard-link deduplication by Dev/Ino),
// R4.1 (exit 0 on success), R4.2 (exit 1 on error, continue processing),
// R5.1 (SIGPIPE handler via pkg/sys.InstallSIGPIPEHandler).
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// duOpts holds the parsed flag state for this invocation.
type duOpts struct {
	humanReadable bool // R2.1: -h
	summary       bool // R2.2: -s
	allFiles      bool // R2.3: -a
	maxDepth      int  // R2.4: -d N / --max-depth=N; -1 means unlimited
	megaBlocks    bool // R2.6: -m (1M blocks)
	grandTotal    bool // R2.7: -c
	apparentSize  bool // R2.8: --apparent-size
}

// inodeKey identifies a unique file by device and inode. R3.2.
type inodeKey struct {
	Dev uint64
	Ino uint64
}

func main() {
	// R5.1: exit 0 on SIGPIPE when stdout is closed by a downstream consumer.
	sys.InstallSIGPIPEHandler()

	opts, args := parseArgs(os.Args[1:])

	// GNU du rejects -a combined with -s.
	if opts.allFiles && opts.summary {
		fmt.Fprintf(os.Stderr, "du: cannot both summarize and show all entries\n")
		fmt.Fprintf(os.Stderr, "Try 'du --help' for more information.\n")
		os.Exit(1)
	}

	// R2.4: -s is equivalent to --max-depth=0.
	if opts.summary && opts.maxDepth < 0 {
		opts.maxDepth = 0
	}

	// R1.1: default to current directory when no arguments given.
	if len(args) == 0 {
		args = []string{"."}
	}

	exitCode := 0
	var grandTotal int64
	// R3.3: single seen map across all arguments for per-invocation dedup.
	seen := make(map[inodeKey]bool)
	// R1.5: process arguments in command-line order.
	for _, arg := range args {
		argBytes, hadErr := duArg(arg, &opts, seen)
		grandTotal += argBytes
		if hadErr {
			exitCode = 1
		}
	}

	// R2.7: print grand total line when -c is given.
	if opts.grandTotal {
		printSize(grandTotal, "total", &opts)
	}

	os.Exit(exitCode)
}

// parseArgs separates flags from path arguments. Returns opts and remaining
// non-flag arguments. Supports -h, -s, -a, -k, -m, -c, -d N, --max-depth=N,
// --apparent-size, and combined short flags (e.g. -sha).
func parseArgs(raw []string) (duOpts, []string) {
	opts := duOpts{maxDepth: -1} // R2.4: -1 means unlimited depth
	var args []string
	endOfFlags := false

	for i := 0; i < len(raw); i++ {
		arg := raw[i]
		if endOfFlags || arg == "-" || !isFlag(arg) {
			args = append(args, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		// Long flags.
		if len(arg) > 2 && arg[:2] == "--" {
			switch {
			case arg == "--human-readable":
				opts.humanReadable = true
			case arg == "--summarize":
				opts.summary = true
			case arg == "--all":
				opts.allFiles = true
			case arg == "--total":
				opts.grandTotal = true
			case arg == "--apparent-size":
				// R2.8: report apparent size instead of block allocation.
				opts.apparentSize = true
			case arg == "--max-depth" || strings.HasPrefix(arg, "--max-depth="):
				var val string
				if strings.Contains(arg, "=") {
					val = arg[len("--max-depth="):]
				} else {
					i++
					if i >= len(raw) {
						printErr("option '--max-depth' requires an argument")
						os.Exit(1)
					}
					val = raw[i]
				}
				n, err := strconv.Atoi(val)
				if err != nil || n < 0 {
					printErr("invalid maximum depth '%s'", val)
					os.Exit(1)
				}
				opts.maxDepth = n
			default:
				printErr("unrecognized option '%s'", arg)
				os.Exit(1)
			}
			continue
		}
		// Short flags: iterate characters after '-'.
		for j := 1; j < len(arg); j++ {
			ch := arg[j]
			switch ch {
			case 'h':
				opts.humanReadable = true
			case 's':
				opts.summary = true
			case 'a':
				opts.allFiles = true
			case 'k':
				// R2.5: -k is 1K blocks (already default), accepted as no-op.
			case 'm':
				// R2.6: -m is 1M blocks.
				opts.megaBlocks = true
			case 'c':
				// R2.7: -c prints grand total.
				opts.grandTotal = true
			case 'd':
				// R2.4: -d N (depth limit). Value is rest of this arg or next arg.
				val := arg[j+1:]
				if val == "" {
					i++
					if i >= len(raw) {
						printErr("option requires an argument -- 'd'")
						os.Exit(1)
					}
					val = raw[i]
				}
				n, err := strconv.Atoi(val)
				if err != nil || n < 0 {
					printErr("invalid maximum depth '%s'", val)
					os.Exit(1)
				}
				opts.maxDepth = n
				j = len(arg) // consumed rest of arg
			default:
				printErr("invalid option -- '%c'", ch)
				os.Exit(1)
			}
		}
	}

	return opts, args
}

// isFlag returns true if arg looks like a flag (starts with '-' and is not just "-").
func isFlag(arg string) bool {
	return len(arg) >= 2 && arg[0] == '-'
}

// fileBytes returns the size contribution for an entry. R2.8: with
// --apparent-size, returns st_size (bytes) for regular files and 0 for
// directories (matching GNU du behavior where directory metadata does not
// contribute to apparent size). Without --apparent-size, returns
// st_blocks * 512 (disk allocation in bytes) for all entry types.
func fileBytes(fi *sys.FileInfo, opts *duOpts) int64 {
	if opts.apparentSize {
		if fi.Mode.IsDir() {
			return 0
		}
		return fi.Size
	}
	return fi.Blocks * 512
}

// countInode returns true if this file's inode has not been seen before.
// R3.1, R3.2: deduplication by (Dev, Ino) pair. Only deduplicates non-directory
// files with Nlink > 1, since those are the only entries that can be hard-linked.
func countInode(fi *sys.FileInfo, seen map[inodeKey]bool) bool {
	if fi.Mode.IsDir() || fi.Nlink <= 1 {
		return true
	}
	key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
	if seen[key] {
		return false
	}
	seen[key] = true
	return true
}

// duArg processes a single command-line argument. Returns total size in bytes
// and whether any error occurred. R1.2: accepts file or directory paths.
func duArg(path string, opts *duOpts, seen map[inodeKey]bool) (int64, bool) {
	fi, err := sys.Lstat(path)
	if err != nil {
		printErr("cannot access '%s': %s", path, errReason(err))
		return 0, true
	}

	if !fi.Mode.IsDir() {
		// R1.2, R3.1: for a file, print its size (dedup if hard-linked).
		var sizeBytes int64
		if countInode(fi, seen) {
			sizeBytes = fileBytes(fi, opts)
		}
		printSize(sizeBytes, path, opts)
		return sizeBytes, false
	}

	total, hadError := walkDir(path, fi, 0, opts, seen)
	return total, hadError
}

// walkDir recursively traverses a directory and prints disk usage for each
// subdirectory. dirFI is the directory entry's own FileInfo.
// depth is the current depth relative to the argument (0 for the argument itself).
// Returns total size in bytes and whether any error occurred.
// R1.1, R1.3, R2.4, R3.1.
func walkDir(dirPath string, dirFI *sys.FileInfo, depth int, opts *duOpts, seen map[inodeKey]bool) (int64, bool) {
	total := fileBytes(dirFI, opts)
	hadError := false

	f, err := os.Open(dirPath)
	if err != nil {
		printErr("cannot read directory '%s': %s", dirPath, errReason(err))
		// R1.3: print directory's own size even on read error.
		if shouldPrintAtDepth(depth, opts) {
			printSize(total, dirPath, opts)
		}
		return total, true
	}
	// Read all entries in filesystem order to match GNU du traversal order.
	names, err := f.Readdirnames(-1)
	f.Close() // best-effort close after reading all entries
	if err != nil {
		printErr("cannot read directory '%s': %s", dirPath, errReason(err))
		hadError = true
	}

	for _, name := range names {
		childPath := dirPath + "/" + name
		childFI, err := sys.Lstat(childPath)
		if err != nil {
			printErr("cannot access '%s': %s", childPath, errReason(err))
			hadError = true
			continue
		}

		if childFI.Mode.IsDir() {
			subtotal, subErr := walkDir(childPath, childFI, depth+1, opts, seen)
			total += subtotal
			if subErr {
				hadError = true
			}
		} else {
			// R3.1: skip already-counted hard-linked files entirely.
			if !countInode(childFI, seen) {
				continue
			}
			// R1.3: accumulate size from Lstat.
			childBytes := fileBytes(childFI, opts)
			total += childBytes
			// R2.3: with -a, print each file if within max-depth.
			if opts.allFiles && shouldPrintAtDepth(depth+1, opts) {
				printSize(childBytes, childPath, opts)
			}
		}
	}

	// R2.4: print directory line only if within max-depth.
	if shouldPrintAtDepth(depth, opts) {
		printSize(total, dirPath, opts)
	}
	return total, hadError
}

// shouldPrintAtDepth returns true if a line at the given depth should be
// printed based on the maxDepth setting. R2.4: depth 0 is the argument itself.
func shouldPrintAtDepth(depth int, opts *duOpts) bool {
	return opts.maxDepth < 0 || depth <= opts.maxDepth
}

// printSize formats and prints a single du output line. The sizeBytes parameter
// is the accumulated size in bytes (either disk allocation or apparent size).
// R1.3, R2.1, R2.6, R2.8.
func printSize(sizeBytes int64, path string, opts *duOpts) {
	if opts.humanReadable {
		// R2.1: human-readable with binary (1024-base) via pkg/format.HumanSize.
		s := format.HumanSize(sizeBytes, format.HumanSizeOpts{Binary: true})
		fmt.Printf("%s\t%s\n", s, path)
	} else if opts.megaBlocks {
		// R2.6: -m reports in 1M blocks, rounding up.
		mblocks := (sizeBytes + 1048575) / 1048576
		fmt.Printf("%d\t%s\n", mblocks, path)
	} else {
		// Default: 1K blocks. Block-based values are always exact multiples of
		// 1024 so truncation is lossless; apparent-size values need ceiling to
		// avoid showing 0 for small files (matching GNU du).
		var kblocks int64
		if opts.apparentSize {
			kblocks = (sizeBytes + 1023) / 1024
		} else {
			kblocks = sizeBytes / 1024
		}
		fmt.Printf("%d\t%s\n", kblocks, path)
	}
}

// printErr writes an error diagnostic to stderr. R4.2.
func printErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "du: "+format+"\n", args...)
}

// errReason extracts the human-readable reason from an OS error,
// capitalizing the first letter to match GNU coreutils strerror() output.
func errReason(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		reason := pathErr.Err.Error()
		if len(reason) > 0 && reason[0] >= 'a' && reason[0] <= 'z' {
			reason = string(reason[0]-32) + reason[1:]
		}
		return reason
	}
	return err.Error()
}
