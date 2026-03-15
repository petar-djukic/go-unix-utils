// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

// Implements prd002-sys R2.3: Darwin-specific Stat_t field extraction.
// Darwin uses st_mtimespec/st_atimespec/st_ctimespec (syscall.Timespec)
// and has different integer widths for Nlink (uint16), Rdev (int32),
// Dev (int32), and Blksize (int32).

package sys

import (
	"syscall"
	"time"
)

// fillFromStat populates platform-specific FileInfo fields from a Darwin Stat_t.
// R2.3: abstracts st_mtimespec (Darwin) vs st_mtim (Linux) divergence.
func fillFromStat(fi *FileInfo, stat *syscall.Stat_t) {
	fi.Nlink = uint64(stat.Nlink)
	fi.Rdev = uint64(stat.Rdev)
	fi.Dev = uint64(stat.Dev)
	fi.Ino = stat.Ino
	fi.Blocks = stat.Blocks
	fi.Blksize = int64(stat.Blksize)
	fi.ModTime = timespecToTime(stat.Mtimespec)
	fi.AccessTime = timespecToTime(stat.Atimespec)
	fi.ChangeTime = timespecToTime(stat.Ctimespec)
}

// timespecToTime converts a syscall.Timespec to time.Time.
func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}
