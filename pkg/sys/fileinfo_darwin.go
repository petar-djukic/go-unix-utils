// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// fillPlatformFields extracts Darwin-specific Stat_t fields into fi.
// Darwin uses st_mtimespec/st_atimespec/st_ctimespec (Timespec structs)
// and st_blocks in 512-byte units. (prd002-sys R2.3)
func fillPlatformFields(fi *FileInfo, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("underlying Sys() is not *syscall.Stat_t")
	}
	fi.Nlink = uint64(stat.Nlink)
	fi.Uid = stat.Uid
	fi.Gid = stat.Gid
	fi.Rdev = uint64(stat.Rdev)
	fi.Dev = uint64(stat.Dev)
	fi.Ino = stat.Ino
	fi.Blocks = stat.Blocks
	fi.Blksize = int64(stat.Blksize)
	fi.ModTime = time.Unix(stat.Mtimespec.Sec, stat.Mtimespec.Nsec)
	fi.AccessTime = time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec)
	fi.ChangeTime = time.Unix(stat.Ctimespec.Sec, stat.Ctimespec.Nsec)
	return nil
}
