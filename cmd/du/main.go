// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/du reports disk usage for files and directory trees.
//
// Implements: prd009-du R1.1, R1.2, R1.3, R1.4, R2.5, R3.1-R3.3, R4.1, R4.2, R5.1
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// inodeKey uniquely identifies a file by device and inode number.
// Used for hard-link deduplication per prd009-du R3.2.
type inodeKey struct {
	Dev uint64
	Ino uint64
}

func main() {
	// R5.1: install SIGPIPE handler so piping to head or similar exits cleanly.
	sys.InstallSIGPIPEHandler()

	// R2.5: -k is accepted without error; 1K blocks is already the default.
	var flagK bool
	flag.BoolVar(&flagK, "k", false, "use 1024-byte (1K) block size (default)")
	flag.Parse()

	args := flag.Args()
	// R1.2: default to current directory when no arguments are given.
	if len(args) == 0 {
		args = []string{"."}
	}

	// R3.3: seen map is shared across all arguments for cross-argument deduplication.
	seen := make(map[inodeKey]struct{})
	exitCode := 0

	for _, arg := range args {
		// R4.2: print error and continue processing remaining arguments on failure.
		if err := runArg(arg, seen); err != nil {
			fmt.Fprintf(os.Stderr, "du: %v\n", err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// runArg processes one command-line argument (file or directory).
// For a regular file it prints its own block count.
// For a directory it recursively walks the tree and prints each subdirectory,
// then prints the root total.
func runArg(path string, seen map[inodeKey]struct{}) error {
	// R1.4: use Lstat so symbolic links are not followed.
	fi, err := sys.Lstat(path)
	if err != nil {
		return err
	}

	if !fi.Mode.IsDir() {
		// Single file argument: count and print its own blocks.
		key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
		var blocks int64
		if _, dup := seen[key]; !dup {
			seen[key] = struct{}{}
			blocks = fi.Blocks
		}
		// R1.3: "SIZE\tPATH\n" format; SIZE is 1K blocks.
		fmt.Printf("%d\t%s\n", toKBlocks(blocks), path)
		return nil
	}

	// Directory: recurse, printing each subdirectory in post-order, then print root.
	total, ok := walkDir(path, seen)
	fmt.Printf("%d\t%s\n", toKBlocks(total), path)
	if !ok {
		return fmt.Errorf("errors encountered while traversing %q", path)
	}
	return nil
}

// walkDir recursively traverses dir, prints a line for each subdirectory in
// post-order (children before parent), and returns the total 512-byte block
// count for dir and all its contents.
//
// R1.1: recursive directory traversal with accumulated usage.
// R1.4: sys.Lstat does not follow symbolic links.
// R3.1-R3.3: hard-link deduplication via shared seen map.
func walkDir(dir string, seen map[inodeKey]struct{}) (total int64, ok bool) {
	// R1.4: Lstat — does not follow symlinks.
	fi, err := sys.Lstat(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: cannot access %q: %v\n", dir, err)
		return 0, false
	}

	key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
	if _, dup := seen[key]; dup {
		// R3.1: directory already counted; skip to avoid double-counting hard links.
		return 0, true
	}
	seen[key] = struct{}{}

	// Include the directory inode's own block allocation.
	total = fi.Blocks
	ok = true

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: cannot read directory %q: %v\n", dir, err)
		return total, false
	}

	for _, entry := range entries {
		childPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			// Recurse into subdirectory; walkDir handles its own dedup.
			subtotal, childOK := walkDir(childPath, seen)
			if !childOK {
				ok = false
			}
			total += subtotal
			// R1.1, R1.3: print subdirectory line after all its children (post-order).
			fmt.Printf("%d\t%s\n", toKBlocks(subtotal), childPath)
		} else {
			// File or symlink: count blocks with deduplication.
			// R1.4: Lstat does not follow the symlink.
			childFi, lstatErr := sys.Lstat(childPath)
			if lstatErr != nil {
				fmt.Fprintf(os.Stderr, "du: cannot access %q: %v\n", childPath, lstatErr)
				ok = false
				continue
			}
			childKey := inodeKey{Dev: childFi.Dev, Ino: childFi.Ino}
			if _, dup := seen[childKey]; !dup {
				seen[childKey] = struct{}{}
				total += childFi.Blocks
			}
		}
	}

	return total, ok
}

// toKBlocks converts a 512-byte block count (st_blocks) to 1024-byte (1K) blocks
// using ceiling division: equivalent to (st_blocks * 512 + 1023) / 1024.
//
// R1.2: default output unit is 1024-byte (1K) blocks.
func toKBlocks(blocks512 int64) int64 {
	return (blocks512 + 1) / 2
}
