// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Attribute preservation for cmd/cp.
//
// Implements prd056-cp R3.1-R3.3.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// preserveSet tracks which file attributes to preserve during copy.
// R3.1: mode, ownership, timestamps via -p.
// R3.3: individual attribute selection via --preserve=ATTR_LIST.
type preserveSet struct {
	mode, ownership, timestamps, links bool
}

// inodeKey identifies a unique file by device and inode number.
type inodeKey struct{ dev, ino uint64 }

// defaultPreserve returns the set for -p (mode, ownership, timestamps).
// R3.1: -p preserves mode, ownership, and timestamps.
func defaultPreserve() preserveSet {
	return preserveSet{mode: true, ownership: true, timestamps: true}
}

// allPreserve returns the full set for --preserve=all.
// R3.2: -a uses --preserve=all.
func allPreserve() preserveSet {
	return preserveSet{
		mode: true, ownership: true, timestamps: true, links: true,
	}
}

// parsePreserveList parses a comma-separated attribute list.
// R3.3: accepts mode, ownership, timestamps, links, all.
func parsePreserveList(list string) (preserveSet, error) {
	if list == "all" {
		return allPreserve(), nil
	}
	var p preserveSet
	for _, attr := range strings.Split(list, ",") {
		switch strings.TrimSpace(attr) {
		case "mode":
			p.mode = true
		case "ownership":
			p.ownership = true
		case "timestamps":
			p.timestamps = true
		case "links":
			p.links = true
		default:
			return p, fmt.Errorf(
				"invalid --preserve attribute '%s'", attr)
		}
	}
	return p, nil
}

// preserveFileAttrs preserves selected attributes from src on dest.
// Returns true if all requested attributes were successfully applied.
func preserveFileAttrs(
	src, dest string, p preserveSet, stderr *os.File,
) bool {
	if !p.mode && !p.ownership && !p.timestamps {
		return true
	}
	fi, err := sys.Stat(src)
	if err != nil {
		printError(stderr, fmt.Sprintf(
			"failed to get attributes of '%s': %s",
			src, stripPathError(err)))
		return false
	}
	return applyAttrs(fi, dest, p, stderr)
}

// preserveDirAttrs preserves attributes on a directory after its entries
// are copied, so that timestamps reflect the original directory.
func preserveDirAttrs(
	src, dest string, p preserveSet, stderr *os.File,
) bool {
	if !p.mode && !p.ownership && !p.timestamps {
		return true
	}
	fi, err := sys.Stat(src)
	if err != nil {
		return true // stat failure on directory after copy is non-fatal
	}
	return applyAttrs(fi, dest, p, stderr)
}

// applyAttrs applies the requested attributes from fi to dest.
func applyAttrs(
	fi *sys.FileInfo, dest string, p preserveSet, stderr *os.File,
) bool {
	ok := true
	if p.mode {
		if err := os.Chmod(dest, fi.Mode.Perm()); err != nil {
			printError(stderr, fmt.Sprintf(
				"preserving permissions for '%s': %s",
				dest, stripPathError(err)))
			ok = false
		}
	}
	if p.ownership {
		if err := os.Lchown(dest, int(fi.Uid), int(fi.Gid)); err != nil {
			printError(stderr, fmt.Sprintf(
				"preserving ownership for '%s': %s",
				dest, stripPathError(err)))
			ok = false
		}
	}
	if p.timestamps {
		if err := os.Chtimes(dest, fi.AccessTime, fi.ModTime); err != nil {
			printError(stderr, fmt.Sprintf(
				"preserving times for '%s': %s",
				dest, stripPathError(err)))
			ok = false
		}
	}
	return ok
}

// preserveSymlinkOwner preserves ownership on a symlink.
func preserveSymlinkOwner(src, dest string, stderr *os.File) bool {
	fi, err := sys.Lstat(src)
	if err != nil {
		return true
	}
	if err := os.Lchown(dest, int(fi.Uid), int(fi.Gid)); err != nil {
		printError(stderr, fmt.Sprintf(
			"preserving ownership for '%s': %s",
			dest, stripPathError(err)))
		return false
	}
	return true
}

// tryHardLink checks if src shares an inode with a previously copied file
// and creates a hard link if so. Returns true if a link was created.
func tryHardLink(src, dest string, inodeMap map[inodeKey]string) bool {
	fi, err := sys.Lstat(src)
	if err != nil || fi.Nlink < 2 {
		return false
	}
	key := inodeKey{dev: fi.Dev, ino: fi.Ino}
	if existing, ok := inodeMap[key]; ok {
		if err := os.Link(existing, dest); err == nil {
			return true
		}
	}
	return false
}

// recordInode records the destination path for a source file's inode.
func recordInode(src, dest string, inodeMap map[inodeKey]string) {
	if inodeMap == nil {
		return
	}
	fi, err := sys.Lstat(src)
	if err != nil {
		return
	}
	key := inodeKey{dev: fi.Dev, ino: fi.Ino}
	if _, ok := inodeMap[key]; !ok {
		inodeMap[key] = dest
	}
}

// setArchiveMode configures options for -a (archive) mode.
// R3.2: -a is equivalent to -dR --preserve=all.
func setArchiveMode(opts *cpOptions) {
	opts.recursive = true
	opts.noDereference = true
	opts.dereference = false
	opts.preserve = allPreserve()
}
