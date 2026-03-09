// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps Darwin and Linux syscalls for file metadata, terminal
// queries, and signal handling. Implements prd002-sys R2.1-R2.3.
package sys

import (
	"fmt"
	"os"
	"time"
)

// FileInfo holds extended file metadata extracted from syscall.Stat_t fields
// not exposed by the standard os.FileInfo interface. (prd002-sys R2.2)
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
// Equivalent to os.Stat but returns a *FileInfo with all Stat_t fields
// populated. (prd002-sys R2.1)
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	return fileInfoFromOS(info)
}

// Lstat returns extended file metadata for path without following symbolic
// links. Equivalent to os.Lstat but returns a *FileInfo with all Stat_t
// fields populated. (prd002-sys R2.1)
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}
	return fileInfoFromOS(info)
}

// fileInfoFromOS converts an os.FileInfo into a *FileInfo by extracting
// platform-specific fields from the underlying syscall.Stat_t.
func fileInfoFromOS(info os.FileInfo) (*FileInfo, error) {
	fi := &FileInfo{
		Mode:    info.Mode(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Info:    info,
	}
	if err := fillPlatformFields(fi, info); err != nil {
		return nil, err
	}
	return fi, nil
}
