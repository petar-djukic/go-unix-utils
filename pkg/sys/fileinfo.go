// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys provides syscall abstractions for go-unix-utils.
// It wraps Darwin and Linux syscalls into a stable, platform-independent API
// so that cmd/ packages do not import syscall or golang.org/x/sys directly.
//
// Implements: prd002-sys R2.1–R2.3 (FileInfo, Stat, Lstat).
package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// FileInfo aggregates all metadata from syscall.Stat_t into a
// platform-independent type. R2.2.
type FileInfo struct {
	// Mode is the file mode and type bits (from syscall.Stat_t.Mode).
	Mode os.FileMode
	// Size is the apparent file size in bytes (st_size).
	Size int64
	// Nlink is the hard-link count (st_nlink).
	Nlink uint64
	// Uid is the owner user ID (st_uid).
	Uid uint32
	// Gid is the owner group ID (st_gid).
	Gid uint32
	// Rdev is the device ID for special files (st_rdev).
	Rdev uint64
	// Dev is the device ID of the containing filesystem (st_dev).
	Dev uint64
	// Ino is the inode number (st_ino).
	Ino uint64
	// Blocks is the number of 512-byte blocks allocated (st_blocks).
	Blocks int64
	// Blksize is the preferred I/O block size (st_blksize).
	Blksize int64
	// ModTime is the modification time (st_mtime / st_mtimespec).
	ModTime time.Time
	// AccessTime is the last access time (st_atime / st_atimespec).
	AccessTime time.Time
	// ChangeTime is the status change time (st_ctime / st_ctimespec).
	ChangeTime time.Time
	// Info is the underlying os.FileInfo for os package compatibility.
	Info os.FileInfo
}

// Stat returns a *FileInfo for the given path, following symbolic links.
// R2.1.
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	return fileInfoFromOS(info)
}

// Lstat returns a *FileInfo for the given path without following symbolic
// links. R2.1.
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}
	return fileInfoFromOS(info)
}

// fileInfoFromOS extracts a *FileInfo from an os.FileInfo by accessing the
// underlying syscall.Stat_t.
func fileInfoFromOS(info os.FileInfo) (*FileInfo, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("cannot extract syscall.Stat_t from os.FileInfo")
	}
	fi := extractFileInfo(stat)
	fi.Mode = info.Mode()
	fi.Size = info.Size()
	fi.Info = info
	return fi, nil
}
