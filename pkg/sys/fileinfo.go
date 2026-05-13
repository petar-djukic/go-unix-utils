// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys provides portable syscall abstractions for terminal queries,
// extended file metadata, and signal handling on Darwin and Linux.
// Implements srd002-sys.
package sys

import (
	"os"
	"time"
)

// FileInfo holds extended file metadata from syscall.Stat_t that is not
// available through os.FileInfo alone.
// R2.2: struct fields match the package contract.
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
// R2.1: calls os.Stat and populates FileInfo from syscall.Stat_t.
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return fileInfoFromOS(info)
}

// Lstat returns extended file metadata for path without following symbolic links.
// R2.1: calls os.Lstat and populates FileInfo from syscall.Stat_t.
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	return fileInfoFromOS(info)
}
