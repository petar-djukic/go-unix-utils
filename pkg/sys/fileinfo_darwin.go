// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"syscall"
	"time"
)

// fillFromStat populates FileInfo fields from a Darwin syscall.Stat_t.
// Darwin uses Mtimespec (not Mtim) for modification time, and field types
// differ from Linux (Nlink is uint16, Dev/Rdev are int32, Blksize is int32).
//
// Implements: prd002-sys R2.3
func fillFromStat(fi *FileInfo, stat *syscall.Stat_t) {
	fi.Nlink = uint64(stat.Nlink)
	fi.Uid = stat.Uid
	fi.Gid = stat.Gid
	fi.Rdev = uint64(stat.Rdev)
	fi.Dev = uint64(stat.Dev)
	fi.Ino = stat.Ino
	fi.Blocks = stat.Blocks
	fi.Blksize = int64(stat.Blksize)
	fi.ModTime = time.Unix(stat.Mtimespec.Sec, stat.Mtimespec.Nsec)
}
