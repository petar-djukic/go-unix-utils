// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys provides syscall abstractions for Darwin and Linux.
// Implements srd002-sys: terminal width, extended file metadata, signal handling.
package sys

import (
	"os"
	"syscall"
	"time"
)

// FileInfo holds extended file metadata from syscall.Stat_t fields not
// exposed by os.FileInfo. See srd002-sys R2.2.
type FileInfo struct {
	Mode       os.FileMode // file mode and type bits (from syscall.Stat_t.Mode)
	Size       int64       // apparent file size in bytes (st_size)
	Nlink      uint64      // hard-link count (st_nlink)
	Uid        uint32      // owner user ID (st_uid)
	Gid        uint32      // owner group ID (st_gid)
	Rdev       uint64      // device ID for special files (st_rdev)
	Dev        uint64      // device ID of the containing filesystem (st_dev)
	Ino        uint64      // inode number (st_ino)
	Blocks     int64       // number of 512-byte blocks allocated (st_blocks)
	Blksize    int64       // preferred I/O block size (st_blksize)
	ModTime    time.Time   // modification time (st_mtime / st_mtimespec)
	AccessTime time.Time   // access time (st_atime / st_atimespec)
	ChangeTime time.Time   // status change time (st_ctime / st_ctimespec)
	Info       os.FileInfo // underlying os.FileInfo for os package compatibility
}

// Stat returns extended file metadata for path, following symbolic links.
// Equivalent to os.Stat but populates all FileInfo fields from syscall.Stat_t.
// See srd002-sys R2.1.
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return fileInfoFromOS(info), nil
}

// Lstat returns extended file metadata for path without following symbolic links.
// Equivalent to os.Lstat but populates all FileInfo fields from syscall.Stat_t.
// See srd002-sys R2.1.
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	return fileInfoFromOS(info), nil
}

// fileInfoFromOS builds a FileInfo from an os.FileInfo, extracting
// extended fields from the underlying syscall.Stat_t.
func fileInfoFromOS(info os.FileInfo) *FileInfo {
	fi := &FileInfo{
		Mode: info.Mode(),
		Size: info.Size(),
		Info: info,
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		populateFromStat(fi, stat)
	}
	return fi
}
