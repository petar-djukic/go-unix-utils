// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys provides syscall abstractions for terminal queries, extended file
// metadata, and signal handling. Implements prd002-sys.
package sys

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// FileInfo holds extended file metadata from syscall.Stat_t that is not
// available through os.FileInfo. It abstracts Darwin/Linux divergence in
// struct field names (st_mtimespec vs st_mtim). (prd002-sys R2.2)
type FileInfo struct {
	Mode       os.FileMode // file mode and type bits
	Size       int64       // apparent file size in bytes (st_size)
	Nlink      uint64      // hard-link count (st_nlink)
	Uid        uint32      // owner user ID (st_uid)
	Gid        uint32      // owner group ID (st_gid)
	Rdev       uint64      // device ID for special files (st_rdev)
	Dev        uint64      // device ID of the containing filesystem (st_dev)
	Ino        uint64      // inode number (st_ino)
	Blocks     int64       // number of 512-byte blocks allocated (st_blocks)
	Blksize    int64       // preferred I/O block size (st_blksize)
	ModTime    time.Time   // modification time (st_mtime)
	AccessTime time.Time   // access time (st_atime)
	ChangeTime time.Time   // status change time (st_ctime)
	Info       os.FileInfo // underlying os.FileInfo for os package compatibility
}

// Stat returns extended file metadata for path, following symbolic links.
// (prd002-sys R2.1)
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	fi := fillFromStat(&st)
	fi.Mode = info.Mode()
	fi.Info = info
	return fi, nil
}

// Lstat returns extended file metadata for path without following symbolic
// links. (prd002-sys R2.1)
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}
	fi := fillFromStat(&st)
	fi.Mode = info.Mode()
	fi.Info = info
	return fi, nil
}
