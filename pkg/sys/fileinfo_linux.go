// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"syscall"
	"time"
)

// fillFromStat populates FileInfo fields from a Linux syscall.Stat_t.
// Linux uses Mtim (not Mtimespec) for modification time. Explicit casts
// handle Nlink (uint32 on arm64, uint64 on amd64) and Blksize (int32 on
// arm64, int64 on amd64) divergence across Linux architectures.
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
	fi.ModTime = time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec)
}
