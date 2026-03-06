// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Sorting comparators for cmd/ls: name, size, time, extension,
// directory-order, and reverse.
//
// Implements: prd010-ls-extended R2 (sort order flags)
// Architecture: docs/ARCHITECTURE.yaml § cmd/
package main

import (
	"sort"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// sortMode determines the primary sort key.
type sortMode int

const (
	sortByName      sortMode = iota // default: byte order (prd008-ls R1.3)
	sortByTime                      // -t: newest first (prd010-ls-extended R2.1)
	sortBySize                      // -S: largest first (prd010-ls-extended R2.2)
	sortByExtension                 // -X: extension alphabetical order
	sortNone                        // -U: directory order (prd010-ls-extended R2.4)
)

// timeField selects which timestamp is used for sorting and display.
type timeField int

const (
	timeModified timeField = iota // default: modification time (st_mtime)
	timeChanged                   // -c: status change time (st_ctime)
	timeAccessed                  // -u: access time (st_atime)
)

// sortEntries sorts entries in place according to the active sort mode.
// R2.4: sortNone skips sorting entirely, preserving directory order.
// R2.3: -r reverses any sort.
func sortEntries(entries []fileEntry, cfg lsConfig) {
	if cfg.sortBy == sortNone || len(entries) < 2 {
		return
	}
	sort.SliceStable(entries, func(i, j int) bool {
		less := compareEntries(entries[i], entries[j], cfg)
		if cfg.reverse {
			return !less
		}
		return less
	})
}

// compareEntries returns true when a should appear before b under the
// active sort mode. Falls back to name comparison when metadata is missing
// or values are equal (tie-breaking by name per R2.1, R2.2).
func compareEntries(a, b fileEntry, cfg lsConfig) bool {
	switch cfg.sortBy {
	case sortByTime:
		at := entryTime(a.info, cfg.timeField)
		bt := entryTime(b.info, cfg.timeField)
		if at.IsZero() || bt.IsZero() {
			return a.name < b.name
		}
		if !at.Equal(bt) {
			return at.After(bt)
		}
		return a.name < b.name
	case sortBySize:
		if a.info == nil || b.info == nil {
			return a.name < b.name
		}
		if a.info.Size != b.info.Size {
			return a.info.Size > b.info.Size
		}
		return a.name < b.name
	case sortByExtension:
		extA := fileExtension(a.name)
		extB := fileExtension(b.name)
		if extA != extB {
			return extA < extB
		}
		return a.name < b.name
	default:
		return a.name < b.name
	}
}

// entryTime returns the selected timestamp for the given file info.
// Returns the zero time when fi is nil.
func entryTime(fi *sys.FileInfo, tf timeField) time.Time {
	if fi == nil {
		return time.Time{}
	}
	switch tf {
	case timeChanged:
		return fi.ChangeTime
	case timeAccessed:
		return fi.AccessTime
	default:
		return fi.ModTime
	}
}

// fileExtension returns the file extension (part after the last dot) for
// sorting purposes. If the name has no dot, or the only dot is at position 0
// (hidden file like ".bashrc"), returns an empty string. This matches GNU ls
// -X behavior where files without extensions sort before files with extensions.
func fileExtension(name string) string {
	for i := len(name) - 1; i > 0; i-- {
		if name[i] == '.' {
			return name[i+1:]
		}
	}
	return ""
}
