// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"syscall"
	"time"
)

// fillFromStat extracts extended metadata fields from a Darwin syscall.Stat_t
// into a FileInfo. Abstracts the Darwin-specific field names and types:
//   - st_mtimespec (Darwin) for modification time
//   - st_nlink is uint16 on Darwin (vs uint64 on Linux amd64)
//   - st_dev and st_rdev are int32 on Darwin (vs uint64 on Linux)
//   - st_blksize is int32 on Darwin (vs int64 on Linux amd64)
//   - st_blocks counts in 512-byte units on both Darwin and Linux
//
// Per prd002-sys R2.3.
func fillFromStat(fi *FileInfo, st *syscall.Stat_t) {
	fi.Nlink = uint64(st.Nlink)
	fi.Uid = st.Uid
	fi.Gid = st.Gid
	fi.Rdev = uint64(st.Rdev)
	fi.Dev = uint64(st.Dev)
	fi.Ino = st.Ino
	fi.Blocks = st.Blocks
	fi.Blksize = int64(st.Blksize)
	fi.ModTime = time.Unix(st.Mtimespec.Sec, st.Mtimespec.Nsec)
}
