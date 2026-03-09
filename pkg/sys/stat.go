// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"fmt"
	"os"
	"syscall"
)

// Stat returns a *FileInfo for path, following symbolic links.
// R2.1: equivalent to os.Stat with extended metadata from syscall.Stat_t.
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	return fileInfoFromOS(info)
}

// Lstat returns a *FileInfo for path without following symbolic links.
// R2.1: equivalent to os.Lstat with extended metadata from syscall.Stat_t.
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}
	return fileInfoFromOS(info)
}

// fileInfoFromOS populates a FileInfo from an os.FileInfo by extracting
// the underlying syscall.Stat_t. R2.2, R2.3.
func fileInfoFromOS(info os.FileInfo) (*FileInfo, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("underlying type is not *syscall.Stat_t")
	}
	fi := &FileInfo{
		Mode:    info.Mode(),
		Size:    info.Size(),
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
	// R2.3: platform-specific time extraction (Darwin vs Linux).
	fillTimes(fi, stat)
	return fi, nil
}
