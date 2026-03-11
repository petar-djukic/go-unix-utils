// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"syscall"
	"time"
)

// setStatFields populates platform-divergent FileInfo fields from syscall.Stat_t
// on Linux. Linux uses uint64 for Dev/Rdev/Nlink and int64 for Blksize, and
// Mtim/Atim/Ctim (instead of Mtimespec/Atimespec/Ctimespec) for timestamps.
//
// R1.4: Abstracts Linux st_mtim field name from Darwin st_mtimespec.
// time.Unix is the equivalent of unix.TimespecToTime per R4.
func setStatFields(fi *FileInfo, raw *syscall.Stat_t) {
	fi.Nlink = raw.Nlink
	fi.Dev = raw.Dev
	fi.Rdev = raw.Rdev
	fi.Blksize = raw.Blksize
	fi.ModTime = time.Unix(raw.Mtim.Sec, raw.Mtim.Nsec)
	fi.AccessTime = time.Unix(raw.Atim.Sec, raw.Atim.Nsec)
	fi.ChangeTime = time.Unix(raw.Ctim.Sec, raw.Ctim.Nsec)
}
