// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"syscall"
	"time"
)

// populateFromStat fills FileInfo fields from a Darwin syscall.Stat_t.
// R1.4: Darwin uses Mtimespec, Atimespec, Ctimespec for time fields.
func populateFromStat(fi *FileInfo, stat *syscall.Stat_t) {
	fi.Nlink = uint64(stat.Nlink)
	fi.Uid = stat.Uid
	fi.Gid = stat.Gid
	fi.Rdev = uint64(uint32(stat.Rdev))
	fi.Dev = uint64(uint32(stat.Dev))
	fi.Ino = stat.Ino
	fi.Blocks = stat.Blocks
	fi.Blksize = int64(stat.Blksize)
	fi.ModTime = timespecToTime(stat.Mtimespec)
	fi.AccessTime = timespecToTime(stat.Atimespec)
	fi.ChangeTime = timespecToTime(stat.Ctimespec)
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}
