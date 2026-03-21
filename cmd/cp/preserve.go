// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd056-cp R3.1, R3.2, R3.3: attribute preservation during copy.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// preserveSet tracks which file attributes to preserve during copy.
// R3.1: -p enables mode, ownership, timestamps.
// R3.2: -a enables all attributes.
// R3.3: --preserve=ATTR_LIST enables selected attributes.
type preserveSet struct {
	mode       bool
	ownership  bool
	timestamps bool
	links      bool
}

// any returns true if any attribute is set for preservation.
func (p preserveSet) any() bool {
	return p.mode || p.ownership || p.timestamps || p.links
}

// preserveDefault returns the -p set: mode, ownership, timestamps. R3.1.
func preserveDefault() preserveSet {
	return preserveSet{mode: true, ownership: true, timestamps: true}
}

// preserveAll returns all attributes for -a/--preserve=all. R3.2.
func preserveAll() preserveSet {
	return preserveSet{mode: true, ownership: true, timestamps: true, links: true}
}

// parsePreserveList parses a comma-separated ATTR_LIST. R3.3.
func parsePreserveList(list string) (preserveSet, error) {
	var ps preserveSet
	for _, attr := range strings.Split(list, ",") {
		switch strings.TrimSpace(attr) {
		case "mode":
			ps.mode = true
		case "ownership":
			ps.ownership = true
		case "timestamps":
			ps.timestamps = true
		case "links":
			ps.links = true
		case "all":
			return preserveAll(), nil
		default:
			return ps, fmt.Errorf("invalid preserve attribute: %s", attr)
		}
	}
	return ps, nil
}

// applyPreservation applies requested metadata from src to dest. R3.1–R3.3.
// Symlink targets are not modified: Chtimes and Chmod follow symlinks,
// so they are skipped when dest is a symlink.
func applyPreservation(ps preserveSet, src, dest string, stderr io.Writer) {
	if !ps.any() {
		return
	}
	fi, err := sys.Lstat(src)
	if err != nil {
		return // best-effort: source metadata unavailable
	}
	isSymlink := fi.Mode&os.ModeSymlink != 0
	if ps.timestamps && !isSymlink {
		// R3.2: preserve modification and access times
		_ = os.Chtimes(dest, fi.AccessTime, fi.ModTime)
	}
	if ps.mode && !isSymlink {
		// R3.1: preserve file permissions
		_ = os.Chmod(dest, fi.Mode.Perm())
	}
	if ps.ownership {
		// R3.3: preserve ownership; silently ignored when non-root
		_ = os.Lchown(dest, int(fi.Uid), int(fi.Gid))
	}
}

// inodeKey identifies a unique file by device and inode number.
type inodeKey struct {
	dev uint64
	ino uint64
}

// inodeTracker maps source inodes to destination paths for hard link
// recreation when preserve.links is true. R3.3.
type inodeTracker struct {
	seen map[inodeKey]string
}

// newInodeTracker creates a new empty tracker.
func newInodeTracker() *inodeTracker {
	return &inodeTracker{seen: make(map[inodeKey]string)}
}

// lookup returns the destination path for a previously copied inode.
func (it *inodeTracker) lookup(dev, ino uint64) (string, bool) {
	if it == nil {
		return "", false
	}
	path, ok := it.seen[inodeKey{dev: dev, ino: ino}]
	return path, ok
}

// record stores a dev+ino to destPath mapping.
func (it *inodeTracker) record(dev, ino uint64, destPath string) {
	if it == nil {
		return
	}
	it.seen[inodeKey{dev: dev, ino: ino}] = destPath
}
