// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd009-du R1.1–R1.5 (directory traversal, size accumulation,
// output format, symlink handling, multiple arguments),
// R2.1 (-h human-readable), R2.2 (-s summary), R2.3 (-a all files),
// R2.4 (-d N/--max-depth=N depth limiting), R2.5 (-k), R2.6 (-m),
// R2.7 (-c grand total).
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
	// R1.5: process arguments in command-line order.
	for _, arg := range args {
		argBlocks, hadErr := duArg(arg, &opts)
		grandTotal += argBlocks
		if hadErr {
			exitCode = 1
		}
	}

	// R2.7: print grand total line when -c is given.
	if opts.grandTotal {
		printSize(grandTotal/2, "total", &opts)
	}

	os.Exit(exitCode)
}

// parseArgs separates flags from path arguments. Returns opts and remaining
// non-flag arguments. Supports -h, -s, -a, -k, -m, -c, -d N, --max-depth=N,
// and combined short flags (e.g. -sha).
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

// duArg processes a single command-line argument. Returns total 512-byte blocks
// and whether any error occurred. R1.2: accepts file or directory paths.
func duArg(path string, opts *duOpts) (int64, bool) {
	fi, err := sys.Lstat(path)
	if err != nil {
		printErr("cannot access '%s': %s", path, errReason(err))
		return 0, true
	}

	if !fi.Mode.IsDir() {
		// R1.2: for a file, print only its block count.
		printSize(fi.Blocks/2, path, opts)
		return fi.Blocks, false
	}

	total, hadError := walkDir(path, fi.Blocks, 0, opts)
	return total, hadError
}

// walkDir recursively traverses a directory and prints disk usage for each
// subdirectory. dirBlocks is the directory entry's own st_blocks value.
// depth is the current depth relative to the argument (0 for the argument itself).
// Returns total 512-byte blocks accumulated and whether any error occurred.
// R1.1, R1.3, R2.4.
func walkDir(dirPath string, dirBlocks int64, depth int, opts *duOpts) (int64, bool) {
	total := dirBlocks
	hadError := false

	f, err := os.Open(dirPath)
	if err != nil {
		printErr("cannot read directory '%s': %s", dirPath, errReason(err))
		// R1.3: print directory's own blocks even on read error.
		if shouldPrintAtDepth(depth, opts) {
			printSize(total/2, dirPath, opts)
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
			subtotal, subErr := walkDir(childPath, childFI.Blocks, depth+1, opts)
			total += subtotal
			if subErr {
				hadError = true
			}
		} else {
			// R1.3: accumulate blocks from st_blocks via Lstat.
			total += childFI.Blocks
			// R2.3: with -a, print each file if within max-depth.
			if opts.allFiles && shouldPrintAtDepth(depth+1, opts) {
				printSize(childFI.Blocks/2, childPath, opts)
			}
		}
	}

	// R2.4: print directory line only if within max-depth.
	if shouldPrintAtDepth(depth, opts) {
		printSize(total/2, dirPath, opts)
	}
	return total, hadError
}

// shouldPrintAtDepth returns true if a line at the given depth should be
// printed based on the maxDepth setting. R2.4: depth 0 is the argument itself.
func shouldPrintAtDepth(depth int, opts *duOpts) bool {
	return opts.maxDepth < 0 || depth <= opts.maxDepth
}

// printSize formats and prints a single du output line. R1.3, R2.1, R2.6.
func printSize(kblocks int64, path string, opts *duOpts) {
	if opts.humanReadable {
		// R2.1: human-readable with binary (1024-base) via pkg/format.HumanSize.
		// HumanSize expects bytes; kblocks are 1K units, so multiply by 1024.
		s := format.HumanSize(kblocks*1024, format.HumanSizeOpts{Binary: true})
		fmt.Printf("%s\t%s\n", s, path)
	} else if opts.megaBlocks {
		// R2.6: -m reports in 1M blocks, rounding up from 1K blocks.
		mblocks := (kblocks + 1023) / 1024
		fmt.Printf("%d\t%s\n", mblocks, path)
	} else {
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
