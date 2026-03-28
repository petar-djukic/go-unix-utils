// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps Darwin and Linux syscalls and signal handling.
//
// Implements prd002-sys R1.5–R1.7, R2.1: FileInfo struct, Stat, Lstat.
package sys

import (
	"os"
	"time"
)

// FileInfo holds extended file metadata that Go's os.FileInfo does not expose,
// including hard-link count, ownership, device IDs, and all three timestamps.
//
// R2.2: all fields match the prd002-sys contract.
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

// Stat returns a FileInfo for the named file, following symlinks.
//
// R2.1: calls os.Stat then extracts syscall.Stat_t fields via fileInfoFromOS.
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return fileInfoFromOS(info)
}

// Lstat returns a FileInfo for the named file without following symlinks.
//
// R2.1: calls os.Lstat to avoid following symlinks.
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	return fileInfoFromOS(info)
}
