// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// fillPlatformFields extracts Linux-specific Stat_t fields into fi.
// Linux uses st_mtim/st_atim/st_ctim (Timespec structs) and st_blocks
// in 512-byte units. (prd002-sys R2.3)
func fillPlatformFields(fi *FileInfo, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("underlying Sys() is not *syscall.Stat_t")
	}
	fi.Nlink = stat.Nlink
	fi.Uid = stat.Uid
	fi.Gid = stat.Gid
	fi.Rdev = stat.Rdev
	fi.Dev = stat.Dev
	fi.Ino = stat.Ino
	fi.Blocks = stat.Blocks
	fi.Blksize = int64(stat.Blksize)
	fi.ModTime = time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec)
	fi.AccessTime = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
	fi.ChangeTime = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
	return nil
}
