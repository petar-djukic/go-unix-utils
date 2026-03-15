// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R2.3: Darwin-specific Stat_t field extraction.
// Darwin uses Mtimespec/Atimespec/Ctimespec (not Mtim/Atim/Ctim) and has
// different integer widths for Nlink (uint16), Dev (int32), Rdev (int32),
// and Blksize (int32).

//go:build darwin

package sys

import (
	"syscall"
	"time"
)

// populateFromStat extracts platform-specific fields from a Darwin
// syscall.Stat_t into the FileInfo struct.
//
// R2.3: abstract Darwin field names and type widths.
func populateFromStat(fi *FileInfo, stat *syscall.Stat_t) {
	fi.Nlink = uint64(stat.Nlink)
	fi.Rdev = uint64(stat.Rdev)
	fi.Dev = uint64(stat.Dev)
	fi.Ino = stat.Ino
	fi.Blocks = stat.Blocks
	fi.Blksize = int64(stat.Blksize)
	fi.ModTime = time.Unix(stat.Mtimespec.Sec, stat.Mtimespec.Nsec)
	fi.AccessTime = time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec)
	fi.ChangeTime = time.Unix(stat.Ctimespec.Sec, stat.Ctimespec.Nsec)
}
