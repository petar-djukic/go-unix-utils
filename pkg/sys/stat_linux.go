// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"syscall"
	"time"
)

// fillFromStat extracts extended metadata fields from a Linux syscall.Stat_t
// into a FileInfo. Abstracts the Linux-specific field names and types:
//   - st_mtim (Linux) for modification time (vs st_mtimespec on Darwin)
//   - st_nlink varies by architecture: uint32 on arm64, uint64 on amd64
//   - st_blksize varies by architecture: int32 on arm64, int64 on amd64
//   - st_blocks counts in 512-byte units on both Darwin and Linux
//
// Explicit casts are used for Nlink and Blksize to handle cross-architecture
// type differences within Linux.
//
// Per prd002-sys R2.3.
func fillFromStat(fi *FileInfo, st *syscall.Stat_t) {
	fi.Nlink = uint64(st.Nlink)
	fi.Uid = st.Uid
	fi.Gid = st.Gid
	fi.Rdev = st.Rdev
	fi.Dev = st.Dev
	fi.Ino = st.Ino
	fi.Blocks = st.Blocks
	fi.Blksize = int64(st.Blksize)
	fi.ModTime = time.Unix(st.Mtim.Sec, st.Mtim.Nsec)
}
