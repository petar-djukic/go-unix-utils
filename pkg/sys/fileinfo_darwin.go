// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"syscall"
	"time"
)

// setStatFields populates platform-divergent FileInfo fields from syscall.Stat_t
// on Darwin. Darwin uses int32 for Dev/Rdev/Blksize, uint16 for Nlink, and
// Mtimespec/Atimespec/Ctimespec (instead of Mtim/Atim/Ctim) for timestamps.
//
// R1.4: Abstracts Darwin st_mtimespec field name from Linux st_mtim.
// time.Unix is the equivalent of unix.TimespecToTime per R4.
func setStatFields(fi *FileInfo, raw *syscall.Stat_t) {
	fi.Nlink = uint64(raw.Nlink)
	fi.Dev = uint64(raw.Dev)
	fi.Rdev = uint64(raw.Rdev)
	fi.Blksize = int64(raw.Blksize)
	fi.ModTime = time.Unix(raw.Mtimespec.Sec, raw.Mtimespec.Nsec)
	fi.AccessTime = time.Unix(raw.Atimespec.Sec, raw.Atimespec.Nsec)
	fi.ChangeTime = time.Unix(raw.Ctimespec.Sec, raw.Ctimespec.Nsec)
}
