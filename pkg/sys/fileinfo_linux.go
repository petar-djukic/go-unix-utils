// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"syscall"
	"time"
)

// populateFromStat fills FileInfo fields from a Linux syscall.Stat_t.
// R1.4: Linux uses Mtim, Atim, Ctim for time fields.
func populateFromStat(fi *FileInfo, stat *syscall.Stat_t) {
	fi.Nlink = stat.Nlink
	fi.Uid = stat.Uid
	fi.Gid = stat.Gid
	fi.Rdev = stat.Rdev
	fi.Dev = stat.Dev
	fi.Ino = stat.Ino
	fi.Blocks = stat.Blocks
	fi.Blksize = stat.Blksize
	fi.ModTime = timespecToTime(stat.Mtim)
	fi.AccessTime = timespecToTime(stat.Atim)
	fi.ChangeTime = timespecToTime(stat.Ctim)
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}
