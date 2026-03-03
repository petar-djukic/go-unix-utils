// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys provides syscall abstractions for Unix utilities, isolating
// Darwin/Linux divergence and syscalls not cleanly exposed by Go's standard
// library. Only pkg/sys may import golang.org/x/sys; cmd/ packages must not.
//
// Implements: prd002-sys R1, R2, R3
// Architecture: docs/ARCHITECTURE.yaml (pkg/sys/ component)
package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// FileInfo holds extended file metadata not available from os.FileInfo.
// Stat_t field name divergence between Darwin (st_mtimespec, uint16 Nlink)
// and Linux (st_mtim, uint64 Nlink) is abstracted by platform-specific
// build files; callers use this struct on both platforms. (prd002-sys R2.2)
type FileInfo struct {
	Mode    os.FileMode // file mode and type bits (from syscall.Stat_t.Mode)
	Size    int64       // apparent file size in bytes (st_size)
	Nlink   uint64      // hard-link count (st_nlink)
	Uid     uint32      // owner user ID (st_uid)
	Gid     uint32      // owner group ID (st_gid)
	Rdev    uint64      // device ID for special files (st_rdev)
	Dev     uint64      // device ID of the containing filesystem (st_dev)
	Ino     uint64      // inode number (st_ino)
	Blocks  int64       // number of 512-byte blocks allocated (st_blocks)
	Blksize int64       // preferred I/O block size (st_blksize)
	ModTime time.Time   // modification time (st_mtime / st_mtimespec)
	Info    os.FileInfo // underlying os.FileInfo for os package compatibility
}

// Stat returns extended file metadata for path, following symbolic links.
// Equivalent to os.Stat but returns *FileInfo with extended fields.
// (prd002-sys R2.1)
func Stat(path string) (*FileInfo, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	return buildFileInfo(fi)
}

// Lstat returns extended file metadata for path without following symbolic
// links. Returns the symlink's own metadata, not the target's.
// Equivalent to os.Lstat but returns *FileInfo with extended fields.
// (prd002-sys R2.1)
func Lstat(path string) (*FileInfo, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}
	return buildFileInfo(fi)
}

// buildFileInfo extracts the underlying syscall.Stat_t from fi and delegates
// to the platform-specific statFields function (stat_darwin.go / stat_linux.go)
// to populate FileInfo, abstracting mtime and field-name divergence. (prd002-sys R2.3)
func buildFileInfo(fi os.FileInfo) (*FileInfo, error) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("unexpected Sys() type %T; expected *syscall.Stat_t", fi.Sys())
	}
	return statFields(fi, st), nil
}
