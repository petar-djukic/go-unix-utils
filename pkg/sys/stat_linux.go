// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

// Linux-specific Stat_t field extraction. Linux uses Mtim (not Mtimespec)
// and uint64 Nlink/Dev/Rdev, matching the FileInfo field types directly.
// (prd002-sys R2.3)

package sys

import (
	"os"
	"syscall"
	"time"
)

// statFields populates a FileInfo from fi and Linux's syscall.Stat_t.
// Explicit casts handle architecture differences within Linux (e.g., arm64
// has uint32 Nlink and int32 Blksize while amd64 has uint64/int64).
// Only Mtim vs Mtimespec differs from Darwin. (prd002-sys R2.3)
func statFields(fi os.FileInfo, st *syscall.Stat_t) *FileInfo {
	return &FileInfo{
		Mode:    fi.Mode(),
		Size:    st.Size,
		Nlink:   uint64(st.Nlink),
		Uid:     st.Uid,
		Gid:     st.Gid,
		Rdev:    uint64(st.Rdev),
		Dev:     uint64(st.Dev),
		Ino:     uint64(st.Ino),
		Blocks:  st.Blocks,
		Blksize: int64(st.Blksize),
		ModTime: time.Unix(st.Mtim.Sec, st.Mtim.Nsec),
		Info:    fi,
	}
}
