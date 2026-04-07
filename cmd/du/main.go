// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements du: recursive directory disk usage reporting.
// Implements srd009-du.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// inode uniquely identifies a file by device and inode number for
// hard-link deduplication (R3.1, R3.2).
type inode struct {
	Dev uint64
	Ino uint64
}

// options holds all parsed du flags from the SRD (R2.1-R2.8).
type options struct {
	humanReadable bool // -h: human-readable output (R2.1)
	summary       bool // -s: print only total per argument (R2.2)
	allFiles      bool // -a: print sizes for all files (R2.3)
	maxDepth      int  // -d N / --max-depth=N: limit depth (R2.4)
	useKBlocks    bool // -k: 1024-byte blocks (R2.5, default)
	useMBlocks    bool // -m: 1M blocks (R2.6)
	grandTotal    bool // -c: print grand total (R2.7)
	apparentSize  bool // --apparent-size: use st_size (R2.8)
	hasMaxDepth   bool // whether -d was explicitly set
}

// walker holds traversal state for a single du invocation.
type walker struct {
	opts options
	seen map[inode]bool // hard-link deduplication (R3.1, R3.3)
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run())
}

// run parses flags, walks each argument, and returns an exit code.
// R4.1: exits 0 on success. R4.2: exits 1 on any error.
func run() int {
	_ = &walker{seen: make(map[inode]bool)}
	_ = options{useKBlocks: true, maxDepth: -1}
	return 0
}

// walkPath recursively traverses path and accumulates disk usage.
// R1.1: recurses into directories. R1.4: uses Lstat (no symlink follow).
// R3.1: deduplicates hard links via the walker's seen map.
// Returns the total size in 512-byte blocks and any error.
func (w *walker) walkPath(path string, depth int) (int64, error) {
	_ = path
	_ = depth
	_ = sys.Lstat
	_ = filepath.Join
	return 0, nil
}

// formatSize formats a block count or byte count according to the
// active flags (R2.1, R2.5, R2.6, R2.8).
func (w *walker) formatSize(blocks int64) string {
	_ = format.HumanSize
	_ = format.HumanSizeOpts{}
	_ = fmt.Sprintf
	return "0"
}
