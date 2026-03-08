// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys provides syscall abstractions for terminal queries, extended file
// metadata, and signal handling. It wraps golang.org/x/sys/unix to abstract
// Darwin/Linux divergence behind a single API surface.
//
// Implements prd002-sys (R1–R3).
package sys

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// FileInfo holds extended file metadata not available from os.FileInfo alone.
// Fields are populated from syscall.Stat_t via Stat or Lstat.
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
// R2.1: equivalent to os.Stat but populates FileInfo from syscall.Stat_t.
func Stat(path string) (*FileInfo, error) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	return fileInfoFromStat(&st, info), nil
}

// Lstat returns extended file metadata for path without following symbolic links.
// R2.1: equivalent to os.Lstat but populates FileInfo from syscall.Stat_t.
func Lstat(path string) (*FileInfo, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}

	return fileInfoFromStat(&st, info), nil
}
