// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/du implements recursive directory disk usage reporting.
// Implements prd009 R1.1, R1.2, R1.3, R1.4, R3.1, R3.2, R3.3, R4.1, R4.2, R5.1.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"."}
	}

	// D3: track seen inodes with map[uint64]map[uint64]bool keyed by (Dev, Ino)
	seen := make(map[uint64]map[uint64]bool)
	exitCode := 0

	// R1.5: process multiple arguments in order
	for _, arg := range args {
		if _, hasErr := duWalk(arg, seen); hasErr {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// duWalk recursively computes disk usage for path.
// Returns total 512-byte blocks and whether any error occurred.
// R1.1: recurses into directories, prints accumulated size.
// R1.4: uses sys.Lstat to avoid following symbolic links.
func duWalk(path string, seen map[uint64]map[uint64]bool) (int64, bool) {
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: cannot access '%s': %v\n", path, err)
		return 0, true
	}

	// R3.1, R3.2: hard-link deduplication via dev+ino
	if isDuplicate(fi, seen) {
		return 0, false
	}

	blocks := fi.Blocks
	if !fi.Mode.IsDir() {
		return blocks, false
	}

	childBlocks, hasErr := walkChildren(path, seen)
	blocks += childBlocks

	// R1.2: convert 512-byte blocks to 1K blocks; R1.3: SIZE\tPATH\n
	fmt.Printf("%d\t%s\n", blocks/2, path)
	return blocks, hasErr
}

// isDuplicate checks whether a file has already been counted.
// R3.1: tracks by dev+ino across the entire invocation.
// R3.3: deduplication is per invocation, not per argument.
func isDuplicate(fi *sys.FileInfo, seen map[uint64]map[uint64]bool) bool {
	if fi.Mode.IsDir() || fi.Nlink <= 1 {
		return false
	}
	inodes, ok := seen[fi.Dev]
	if !ok {
		inodes = make(map[uint64]bool)
		seen[fi.Dev] = inodes
	}
	if inodes[fi.Ino] {
		return true
	}
	inodes[fi.Ino] = true
	return false
}

// walkChildren reads directory entries and accumulates their disk usage.
func walkChildren(dir string, seen map[uint64]map[uint64]bool) (int64, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: cannot read directory '%s': %v\n", dir, err)
		return 0, true
	}

	var total int64
	hasErr := false
	for _, e := range entries {
		// Concatenate to preserve path prefix (e.g., "./" for "." argument)
		childPath := dir + "/" + e.Name()
		childBlocks, childErr := duWalk(childPath, seen)
		total += childBlocks
		if childErr {
			hasErr = true
		}
	}
	return total, hasErr
}
