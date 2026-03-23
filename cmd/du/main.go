// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd009-du: Recursive Directory Disk Usage.
// Covers R1.1-R1.5 (default traversal, block counting, output format,
// symlink handling, multiple arguments),
// R2.1 (-a/--all flag for reporting all files),
// R3.1-R3.3 (hard-link deduplication via device+inode tracking),
// R4.1-R4.2 (exit codes with error continuation),
// R5.1 (SIGPIPE handler installation).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// devIno is the hard-link deduplication key.
// R3.2: keyed by (Dev, Ino) pair from sys.FileInfo.
type devIno struct {
	Dev uint64
	Ino uint64
}

// duConfig holds parsed command-line options.
type duConfig struct {
	all  bool     // R2.1: -a/--all reports all files
	args []string // positional arguments; defaults to ["."]
}

func main() {
	// R5.1: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %s\n", err)
		os.Exit(1)
	}

	os.Exit(run(cfg))
}

// parseArgs extracts flags and positional arguments.
// D5: supports both short (-a) and long (--all) flag forms.
func parseArgs(args []string) (duConfig, error) {
	var cfg duConfig
	if err := parseFlagsAndArgs(args, &cfg); err != nil {
		return duConfig{}, err
	}
	if len(cfg.args) == 0 {
		cfg.args = []string{"."}
	}
	return cfg, nil
}

// parseFlagsAndArgs iterates over args, populating cfg fields.
// Returns an error for unrecognized options, nil on success.
func parseFlagsAndArgs(args []string, cfg *duConfig) error {
	for i, arg := range args {
		if arg == "--" {
			cfg.args = append(cfg.args, args[i+1:]...)
			return nil
		}
		if arg == "-a" || arg == "--all" {
			cfg.all = true
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			return fmt.Errorf("unrecognized option '%s'", arg)
		}
		cfg.args = append(cfg.args, arg)
	}
	return nil
}

// run processes all arguments and returns the exit code.
// R1.5: processes arguments in command-line order.
// R3.3: shared seen map across all arguments.
func run(cfg duConfig) int {
	exitCode := 0
	seen := make(map[devIno]bool)
	for _, arg := range cfg.args {
		walkArg(arg, cfg.all, seen, &exitCode)
	}
	return exitCode
}

// walkArg processes a single top-level command-line argument.
// R1.4: uses Lstat to avoid following symlinks.
func walkArg(path string, all bool, seen map[devIno]bool, code *int) {
	fi, err := sys.Lstat(path)
	if err != nil {
		reportError("cannot access", path, err)
		*code = 1
		return
	}
	if fi.Mode.IsDir() {
		walkDir(path, fi, all, seen, code)
		return
	}
	// Top-level file arguments are always printed.
	blocks, _ := entryBlocks(fi, seen)
	printEntry(blocks, path)
}

// walkDir recursively traverses a directory and prints accumulated sizes.
// R1.1: prints one line per subdirectory with accumulated block count.
func walkDir(path string, fi *sys.FileInfo, all bool, seen map[devIno]bool, code *int) int64 {
	total, _ := entryBlocks(fi, seen)
	entries, err := os.ReadDir(path)
	if err != nil {
		reportError("cannot read directory", path, err)
		*code = 1
	}
	for _, e := range entries {
		child := joinPath(path, e.Name())
		total += walkChild(child, all, seen, code)
	}
	printEntry(total, path)
	return total
}

// walkChild processes a single entry during directory traversal.
// R2.1: prints file entries only when all (-a) is true.
// R3.1: skips already-seen inodes entirely (no output, no size).
func walkChild(path string, all bool, seen map[devIno]bool, code *int) int64 {
	fi, err := sys.Lstat(path)
	if err != nil {
		reportError("cannot access", path, err)
		*code = 1
		return 0
	}
	if fi.Mode.IsDir() {
		return walkDir(path, fi, all, seen, code)
	}
	blocks, isNew := entryBlocks(fi, seen)
	if isNew && all {
		printEntry(blocks, path)
	}
	return blocks
}

// entryBlocks returns the 512-byte block count for an entry, with dedup.
// R3.1: returns (0, false) if the inode was already counted.
// R3.2: deduplication key is (Dev, Ino) pair.
func entryBlocks(fi *sys.FileInfo, seen map[devIno]bool) (int64, bool) {
	key := devIno{fi.Dev, fi.Ino}
	if seen[key] {
		return 0, false
	}
	seen[key] = true
	return fi.Blocks, true
}

// joinPath concatenates parent and child preserving the parent prefix.
// Unlike filepath.Join, this does not clean "." prefixes away.
func joinPath(parent, child string) string {
	return parent + "/" + child
}

// printEntry prints a du output line converting 512-byte blocks to 1K blocks.
// R1.2: size in 1024-byte blocks = blocks_512 / 2.
// R1.3: format is "SIZE\tPATH\n".
func printEntry(blocks512 int64, path string) {
	fmt.Printf("%d\t%s\n", blocks512/2, path)
}

// reportError prints a diagnostic to stderr.
// R4.2: continues processing after errors.
func reportError(action, path string, err error) {
	msg := err.Error()
	if pe, ok := err.(*os.PathError); ok {
		msg = pe.Err.Error()
	}
	fmt.Fprintf(os.Stderr, "du: %s '%s': %s\n", action, path, msg)
}
