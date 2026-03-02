//go:build darwin

// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd002-sys (R2.3)
package sys

import (
	"syscall"
	"time"
)

// extractMetadata populates FileMetadata from Darwin's syscall.Stat_t.
// Darwin uses Mtimespec/Atimespec for timestamps.
func extractMetadata(st *syscall.Stat_t) FileMetadata {
	return FileMetadata{
		ModTime: time.Unix(st.Mtimespec.Sec, st.Mtimespec.Nsec),
		ATime:   time.Unix(st.Atimespec.Sec, st.Atimespec.Nsec),
		Blocks:  st.Blocks,
		Blksize: int64(st.Blksize),
		Dev:     uint64(st.Dev),
		Ino:     st.Ino,
		Nlink:   uint64(st.Nlink),
		Uid:     st.Uid,
		Gid:     st.Gid,
	}
}
