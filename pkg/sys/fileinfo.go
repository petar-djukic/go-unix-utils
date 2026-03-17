// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys provides portable syscall abstractions for go-unix-utils.
// It wraps Darwin and Linux divergences behind a single API so cmd/ packages
// do not import syscall or golang.org/x/sys directly.
//
// Implements prd002-sys R2.1-R2.3: FileInfo struct, Stat, and Lstat.
package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// FileInfo holds extended file metadata not available from os.FileInfo.
// Fields are populated from the underlying syscall.Stat_t via platform-specific
// extraction in fileinfo_darwin.go or fileinfo_linux.go.
//
// R2.2: FileInfo struct with all specified fields.
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
	// ChangeTime is the last status change time (st_ctime / st_ctimespec).
	ChangeTime time.Time
	// Info is the underlying os.FileInfo for os package compatibility.
	Info os.FileInfo
}

// Stat calls os.Stat and populates a FileInfo from the underlying syscall.Stat_t.
// It follows symbolic links (equivalent to os.Stat).
//
// R2.1: Stat follows symlinks and returns *FileInfo populated from Stat_t.
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return fileInfoFromOS(info)
}

// Lstat calls os.Lstat and populates a FileInfo from the underlying syscall.Stat_t.
// It does not follow symbolic links, preserving symlink metadata.
//
// R2.1: Lstat does not follow symlinks and returns *FileInfo with symlink metadata.
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	return fileInfoFromOS(info)
}

// fileInfoFromOS extracts extended metadata from an os.FileInfo by type-asserting
// the underlying Sys() value to *syscall.Stat_t. Platform-specific field extraction
// is delegated to fillFromStat (defined in fileinfo_darwin.go / fileinfo_linux.go).
func fileInfoFromOS(info os.FileInfo) (*FileInfo, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("sys.fileInfoFromOS: unsupported platform: Sys() is not *syscall.Stat_t")
	}
	fi := &FileInfo{
		Mode: info.Mode(),
		Size: stat.Size,
		Uid:  stat.Uid,
		Gid:  stat.Gid,
		Info: info,
	}
	// R2.3: platform-specific field extraction abstracts Darwin/Linux divergence.
	fillFromStat(fi, stat)
	return fi, nil
}
