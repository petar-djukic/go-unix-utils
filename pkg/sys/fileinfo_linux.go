// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R2.3: Linux-specific Stat_t field extraction.
// Linux uses Mtim/Atim/Ctim (not Mtimespec/Atimespec/Ctimespec).

//go:build linux

package sys

import (
	"syscall"
	"time"
)

// populateFromStat extracts platform-specific fields from a Linux
// syscall.Stat_t into the FileInfo struct.
//
// R2.3: abstract Linux field names.
func populateFromStat(fi *FileInfo, stat *syscall.Stat_t) {
	fi.Nlink = stat.Nlink
	fi.Rdev = stat.Rdev
	fi.Dev = stat.Dev
	fi.Ino = stat.Ino
	fi.Blocks = stat.Blocks
	fi.Blksize = stat.Blksize
	fi.ModTime = time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec)
	fi.AccessTime = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
	fi.ChangeTime = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
}
