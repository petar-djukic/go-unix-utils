// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"syscall"
	"time"
)

// extractFileInfo converts Linux-specific syscall.Stat_t fields to the
// platform-independent FileInfo. Linux uses Mtim/Atim/Ctim for timestamps
// and uint64 for Nlink. R2.3.
func extractFileInfo(stat *syscall.Stat_t) *FileInfo {
	return &FileInfo{
		Nlink:      stat.Nlink,
		Uid:        stat.Uid,
		Gid:        stat.Gid,
		Rdev:       stat.Rdev,
		Dev:        stat.Dev,
		Ino:        stat.Ino,
		Blocks:     stat.Blocks,
		Blksize:    stat.Blksize,
		ModTime:    time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec),
		AccessTime: time.Unix(stat.Atim.Sec, stat.Atim.Nsec),
		ChangeTime: time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec),
	}
}
