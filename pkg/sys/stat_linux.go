//go:build linux

// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd002-sys (R2.3)
package sys

import (
	"syscall"
	"time"
)

// extractMetadata populates FileMetadata from Linux's syscall.Stat_t.
// Linux uses Mtim/Atim for timestamps.
func extractMetadata(st *syscall.Stat_t) FileMetadata {
	return FileMetadata{
		ModTime: time.Unix(st.Mtim.Sec, st.Mtim.Nsec),
		ATime:   time.Unix(st.Atim.Sec, st.Atim.Nsec),
		Blocks:  st.Blocks,
		Blksize: int64(st.Blksize),
		Dev:     uint64(st.Dev),
		Ino:     uint64(st.Ino),
		Nlink:   uint64(st.Nlink),
		Uid:     st.Uid,
		Gid:     st.Gid,
	}
}
