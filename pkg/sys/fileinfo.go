// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps Darwin and Linux syscalls into a portable API so that
// cmd/ packages never import platform-specific packages directly.
//
// Implements: prd002-sys R1 (terminal size), R2 (extended file metadata)
package sys

import (
	"os"
	"syscall"
	"time"
)

// FileInfo holds extended file metadata extracted from syscall.Stat_t.
// Callers use this type to access fields that os.FileInfo does not expose,
// such as hard-link count, device and inode numbers, and physical block
// counts. Platform-specific Stat_t field extraction is handled by
// fillFromStat in the build-tagged files.
//
// Implements: prd002-sys R2.2
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

// Lstat returns extended file metadata without following symbolic links.
// It wraps os.Lstat and populates the FileInfo fields from the
// platform-specific syscall.Stat_t.
//
// Implements: prd002-sys R2.1, R2.3
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	fi := &FileInfo{
		Mode: info.Mode(),
		Size: info.Size(),
		Info: info,
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fi, nil
	}

	fillFromStat(fi, stat)
	return fi, nil
}
