// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys provides syscall abstractions for Darwin and Linux, exposing
// terminal queries, extended file metadata, and signal handling.
// Implements prd002-sys (R1, R2, R3).
package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// FileInfo holds extended file metadata not available from os.FileInfo.
// Fields are populated from syscall.Stat_t, abstracting Darwin/Linux divergence.
// R2.2: struct definition matches the package contract exactly.
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
	ChangeTime time.Time   // change time (st_ctime / st_ctimespec)
	Info       os.FileInfo // underlying os.FileInfo for os package compatibility
}

// Stat returns extended file metadata for path, following symbolic links.
// R2.1: equivalent to os.Stat with additional syscall.Stat_t fields.
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	return fileInfoFromOS(info)
}

// Lstat returns extended file metadata for path without following symbolic links.
// R2.1: equivalent to os.Lstat with additional syscall.Stat_t fields.
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}
	return fileInfoFromOS(info)
}

// fileInfoFromOS extracts syscall.Stat_t fields from an os.FileInfo.
func fileInfoFromOS(info os.FileInfo) (*FileInfo, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("underlying stat type is %T, not *syscall.Stat_t", info.Sys())
	}

	fi := &FileInfo{
		Mode:    info.Mode(),
		Size:    stat.Size,
		Nlink:   uint64(stat.Nlink),
		Uid:     stat.Uid,
		Gid:     stat.Gid,
		Rdev:    uint64(stat.Rdev),
		Dev:     uint64(stat.Dev),
		Ino:     stat.Ino,
		Blocks:  stat.Blocks,
		Blksize: int64(stat.Blksize),
		Info:    info,
	}

	fillTimes(fi, stat)

	return fi, nil
}
