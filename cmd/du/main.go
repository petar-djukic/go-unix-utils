// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd009-du R1.1–R1.5 (directory traversal, size accumulation,
// output format, symlink handling, multiple arguments),
// R2.1 (-h human-readable), R2.2 (-s summary), R2.3 (-a all files).
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// duOpts holds the parsed flag state for this invocation.
type duOpts struct {
	humanReadable bool // R2.1: -h
	summary       bool // R2.2: -s
	allFiles      bool // R2.3: -a
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

	// R1.1: default to current directory when no arguments given.
	if len(args) == 0 {
		args = []string{"."}
	}

	exitCode := 0
	// R1.5: process arguments in command-line order.
	for _, arg := range args {
		if duArg(arg, &opts) {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// parseArgs separates flags from path arguments. Returns opts and remaining
// non-flag arguments. Supports -h, -s, -a and combined short flags (e.g. -sha).
func parseArgs(raw []string) (duOpts, []string) {
	var opts duOpts
	var args []string
	endOfFlags := false

	for _, arg := range raw {
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
			switch arg {
			case "--human-readable":
				opts.humanReadable = true
			case "--summarize":
				opts.summary = true
			case "--all":
				opts.allFiles = true
			default:
				printErr("unrecognized option '%s'", arg)
				os.Exit(1)
			}
			continue
		}
		// Short flags: iterate characters after '-'.
		for _, ch := range arg[1:] {
			switch ch {
			case 'h':
				opts.humanReadable = true
			case 's':
				opts.summary = true
			case 'a':
				opts.allFiles = true
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

// duArg processes a single command-line argument. Returns true if any error
// occurred. R1.2: accepts file or directory paths.
func duArg(path string, opts *duOpts) bool {
	fi, err := sys.Lstat(path)
	if err != nil {
		printErr("cannot access '%s': %s", path, errReason(err))
		return true
	}

	if !fi.Mode.IsDir() {
		// R1.2: for a file, print only its block count.
		printSize(fi.Blocks/2, path, opts)
		return false
	}

	total, hadError := walkDir(path, fi.Blocks, opts)
	// R2.2: with -s, only the top-level argument line is printed.
	if opts.summary {
		printSize(total/2, path, opts)
	}
	return hadError
}

// walkDir recursively traverses a directory and prints disk usage for each
// subdirectory. dirBlocks is the directory entry's own st_blocks value.
// Returns total 512-byte blocks accumulated and whether any error occurred.
// R1.1, R1.3.
func walkDir(dirPath string, dirBlocks int64, opts *duOpts) (int64, bool) {
	total := dirBlocks
	hadError := false

	f, err := os.Open(dirPath)
	if err != nil {
		printErr("cannot read directory '%s': %s", dirPath, errReason(err))
		// R1.3: print directory's own blocks even on read error.
		printSize(total/2, dirPath, opts)
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
			subtotal, subErr := walkDir(childPath, childFI.Blocks, opts)
			total += subtotal
			if subErr {
				hadError = true
			}
		} else {
			// R1.3: accumulate blocks from st_blocks via Lstat.
			total += childFI.Blocks
			// R2.3: with -a, print each file (unless -s suppresses it).
			if opts.allFiles && !opts.summary {
				printSize(childFI.Blocks/2, childPath, opts)
			}
		}
	}

	// R2.2: with -s, only the top-level argument prints (handled in duArg).
	// For subdirectories during -s traversal, skip printing.
	if !opts.summary {
		// R1.3: size in 1K blocks. R1.4: format is SIZE\tPATH\n.
		printSize(total/2, dirPath, opts)
	}
	return total, hadError
}

// printSize formats and prints a single du output line. R1.3, R2.1.
func printSize(kblocks int64, path string, opts *duOpts) {
	if opts.humanReadable {
		// R2.1: human-readable with binary (1024-base) via pkg/format.HumanSize.
		// HumanSize expects bytes; kblocks are 1K units, so multiply by 1024.
		s := format.HumanSize(kblocks*1024, format.HumanSizeOpts{Binary: true})
		fmt.Printf("%s\t%s\n", s, path)
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
