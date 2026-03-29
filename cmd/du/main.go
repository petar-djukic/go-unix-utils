// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/du implements recursive directory disk usage reporting.
// Implements prd009 R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.5, R2.6, R2.8,
// R3.1, R3.2, R3.3, R4.1, R4.2, R5.1.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// duOptions holds command-line flag values for du.
type duOptions struct {
	blockSize     int64
	humanReadable bool
	apparentSize  bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts := parseFlags()
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	seen := make(map[uint64]map[uint64]bool)
	exitCode := 0

	// R1.5: process multiple arguments in order
	for _, arg := range args {
		if _, hasErr := duWalk(arg, seen, opts); hasErr {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// parseFlags parses du command-line flags and returns options.
// R2.5: -k (1024-byte blocks, default).
// R2.6: -m (1M blocks).
// R2.1: -h/--human-readable (binary human-readable).
// R2.8: --apparent-size (report st_size instead of st_blocks).
func parseFlags() duOptions {
	opts := duOptions{blockSize: 1024}
	var kFlag, mFlag bool
	flag.BoolVar(&kFlag, "k", false, "display sizes in 1024-byte blocks")
	flag.BoolVar(&mFlag, "m", false, "display sizes in 1048576-byte blocks")
	flag.BoolVar(&opts.humanReadable, "h", false, "human-readable output")
	flag.BoolVar(&opts.humanReadable, "human-readable", false, "human-readable output")
	flag.BoolVar(&opts.apparentSize, "apparent-size", false, "print apparent sizes")
	// TODO: --si is listed in prd009 non_goals; skipping per E6
	// TODO: -b / --bytes is listed in prd009 non_goals; skipping per E6
	// TODO: --block-size=SIZE is listed in prd009 non_goals; skipping per E6
	flag.Parse()
	if mFlag {
		opts.blockSize = 1048576
	}
	return opts
}

// fileRawBytes returns the size of a file in bytes.
// R2.8: --apparent-size uses fi.Size; default uses fi.Blocks * 512.
func fileRawBytes(fi *sys.FileInfo, apparentSize bool) int64 {
	if apparentSize {
		return fi.Size
	}
	return fi.Blocks * 512
}

// formatSize formats a raw byte count for display.
// R2.1: -h uses format.HumanSize with Binary=true.
// R2.5, R2.6: otherwise divides by blockSize with ceiling.
func formatSize(rawBytes int64, opts duOptions) string {
	if opts.humanReadable {
		return format.HumanSize(rawBytes, format.HumanSizeOpts{Binary: true})
	}
	return fmt.Sprintf("%d", ceilDiv(rawBytes, opts.blockSize))
}

// ceilDiv returns the ceiling of a/b for positive a.
func ceilDiv(a, b int64) int64 {
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

// duWalk recursively computes disk usage for path.
// Returns total raw bytes and whether any error occurred.
// R1.1: recurses into directories, prints accumulated size.
// R1.4: uses sys.Lstat to avoid following symbolic links.
func duWalk(path string, seen map[uint64]map[uint64]bool, opts duOptions) (int64, bool) {
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: cannot access '%s': %v\n", path, err)
		return 0, true
	}

	if isDuplicate(fi, seen) {
		return 0, false
	}

	rawBytes := fileRawBytes(fi, opts.apparentSize)
	if !fi.Mode.IsDir() {
		return rawBytes, false
	}

	childBytes, hasErr := walkChildren(path, seen, opts)
	rawBytes += childBytes

	// R1.3: SIZE\tPATH\n
	fmt.Printf("%s\t%s\n", formatSize(rawBytes, opts), path)
	return rawBytes, hasErr
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
func walkChildren(dir string, seen map[uint64]map[uint64]bool, opts duOptions) (int64, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: cannot read directory '%s': %v\n", dir, err)
		return 0, true
	}

	var total int64
	hasErr := false
	for _, e := range entries {
		childPath := dir + "/" + e.Name()
		childBytes, childErr := duWalk(childPath, seen, opts)
		total += childBytes
		if childErr {
			hasErr = true
		}
	}
	return total, hasErr
}
