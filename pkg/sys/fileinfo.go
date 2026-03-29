// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"time"
)

// FileInfo holds extended file metadata populated from syscall.Stat_t.
//
// R1.1 (prd002): fields cover mode, size, link count, ownership, device,
// inode, block counts, and timestamps including access and change times.
// R2.2: struct layout matches the package contract.
type FileInfo struct {
	Mode       os.FileMode
	Size       int64
	Nlink      uint64
	Uid        uint32
	Gid        uint32
	Rdev       uint64
	Dev        uint64
	Ino        uint64
	Blocks     int64
	Blksize    int64
	ModTime    time.Time
	AccessTime time.Time
	ChangeTime time.Time
	Info       os.FileInfo
}

// Stat returns extended file metadata for path, following symbolic links.
//
// R2.1: equivalent to os.Stat but populates FileInfo from syscall.Stat_t.
func Stat(path string) (*FileInfo, error) {
	panic("not implemented")
}

// Lstat returns extended file metadata for path without following symbolic links.
//
// R2.1: equivalent to os.Lstat but populates FileInfo from syscall.Stat_t.
func Lstat(path string) (*FileInfo, error) {
	panic("not implemented")
}
