// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// fromStat converts a unix.Stat_t to FileInfo on Darwin. R2.3.
// golang.org/x/sys/unix normalizes Darwin Stat_t field names to Mtim/Atim/Ctim.
// Darwin-specific type widths: Dev and Rdev are int32, Nlink is uint16, Blksize is int32.
func fromStat(s *unix.Stat_t, info os.FileInfo) *FileInfo {
	return &FileInfo{
		Mode:       unixModeToFileMode(uint32(s.Mode)),
		Size:       s.Size,
		Nlink:      uint64(s.Nlink),
		Uid:        s.Uid,
		Gid:        s.Gid,
		Rdev:       uint64(s.Rdev),
		Dev:        uint64(s.Dev),
		Ino:        s.Ino,
		Blocks:     s.Blocks,
		Blksize:    int64(s.Blksize),
		ModTime:    time.Unix(s.Mtim.Sec, s.Mtim.Nsec),
		AccessTime: time.Unix(s.Atim.Sec, s.Atim.Nsec),
		ChangeTime: time.Unix(s.Ctim.Sec, s.Ctim.Nsec),
		Info:       info,
	}
}
