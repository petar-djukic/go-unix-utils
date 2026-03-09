// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"syscall"
	"time"
)

// extractFileInfo converts Darwin-specific syscall.Stat_t fields to the
// platform-independent FileInfo. Darwin uses Mtimespec/Atimespec/Ctimespec
// for timestamps. R2.3.
func extractFileInfo(stat *syscall.Stat_t) *FileInfo {
	return &FileInfo{
		Nlink:      uint64(stat.Nlink),
		Uid:        stat.Uid,
		Gid:        stat.Gid,
		Rdev:       uint64(stat.Rdev),
		Dev:        uint64(stat.Dev),
		Ino:        stat.Ino,
		Blocks:     stat.Blocks,
		Blksize:    int64(stat.Blksize),
		ModTime:    time.Unix(stat.Mtimespec.Sec, stat.Mtimespec.Nsec),
		AccessTime: time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec),
		ChangeTime: time.Unix(stat.Ctimespec.Sec, stat.Ctimespec.Nsec),
	}
}
