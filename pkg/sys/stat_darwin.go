// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

// Darwin-specific Stat_t field extraction. Darwin uses Mtimespec (not Mtim),
// uint16 Nlink, and int32 Dev/Rdev; all are cast to the platform-independent
// types in FileInfo. (prd002-sys R2.3)

package sys

import (
	"os"
	"syscall"
	"time"
)

// statFields populates a FileInfo from fi and Darwin's syscall.Stat_t.
// Casts Darwin-specific types (uint16 Nlink, int32 Dev/Rdev/Blksize) to the
// portable uint64/int64 types in FileInfo. (prd002-sys R2.3)
func statFields(fi os.FileInfo, st *syscall.Stat_t) *FileInfo {
	return &FileInfo{
		Mode:    fi.Mode(),
		Size:    st.Size,
		Nlink:   uint64(st.Nlink),
		Uid:     st.Uid,
		Gid:     st.Gid,
		Rdev:    uint64(st.Rdev),
		Dev:     uint64(st.Dev),
		Ino:     st.Ino,
		Blocks:  st.Blocks,
		Blksize: int64(st.Blksize),
		ModTime: time.Unix(st.Mtimespec.Sec, st.Mtimespec.Nsec),
		Info:    fi,
	}
}
