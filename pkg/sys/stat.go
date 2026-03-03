// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// FileInfo holds extended file metadata from syscall.Stat_t not available
// from os.FileInfo (prd002-sys R2.2).
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

// Stat returns extended file metadata for path, following symbolic links
// (prd002-sys R2.1).
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return newFileInfo(info)
}

// Lstat returns extended file metadata for path without following symbolic
// links (prd002-sys R2.1).
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	return newFileInfo(info)
}

// newFileInfo extracts syscall.Stat_t from os.FileInfo and populates a
// FileInfo with platform-independent and platform-specific fields.
func newFileInfo(info os.FileInfo) (*FileInfo, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("extracting syscall.Stat_t: unsupported platform")
	}
	fi := &FileInfo{
		Mode: info.Mode(),
		Size: info.Size(),
		Info: info,
	}
	fillFromStat(fi, st)
	return fi, nil
}
