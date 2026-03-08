// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// fromStat converts a unix.Stat_t to FileInfo on Linux. R2.3:
// Linux uses Mtim, Atim, Ctim (not Mtimespec, Atimespec, Ctimespec).
// Linux Nlink is uint64 (Darwin is uint16).
func fromStat(s *unix.Stat_t, info os.FileInfo) *FileInfo {
	return &FileInfo{
		Mode:       unixModeToFileMode(s.Mode),
		Size:       s.Size,
		Nlink:      s.Nlink,
		Uid:        s.Uid,
		Gid:        s.Gid,
		Rdev:       s.Rdev,
		Dev:        s.Dev,
		Ino:        s.Ino,
		Blocks:     s.Blocks,
		Blksize:    int64(s.Blksize),
		ModTime:    time.Unix(s.Mtim.Sec, s.Mtim.Nsec),
		AccessTime: time.Unix(s.Atim.Sec, s.Atim.Nsec),
		ChangeTime: time.Unix(s.Ctim.Sec, s.Ctim.Nsec),
		Info:       info,
	}
}
