// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

// Implements prd002-sys R2.3: Linux-specific Stat_t field extraction.
// Linux uses st_mtim/st_atim/st_ctim (syscall.Timespec) and has
// uint64 widths for Nlink, Rdev, Dev on amd64.

package sys

import (
	"syscall"
	"time"
)

// fillFromStat populates platform-specific FileInfo fields from a Linux Stat_t.
// R2.3: abstracts st_mtim (Linux) vs st_mtimespec (Darwin) divergence.
func fillFromStat(fi *FileInfo, stat *syscall.Stat_t) {
	fi.Nlink = stat.Nlink
	fi.Rdev = stat.Rdev
	fi.Dev = stat.Dev
	fi.Ino = stat.Ino
	fi.Blocks = stat.Blocks
	fi.Blksize = int64(stat.Blksize)
	fi.ModTime = timespecToTime(stat.Mtim)
	fi.AccessTime = timespecToTime(stat.Atim)
	fi.ChangeTime = timespecToTime(stat.Ctim)
}

// timespecToTime converts a syscall.Timespec to time.Time.
func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}
