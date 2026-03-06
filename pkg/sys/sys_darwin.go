// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"syscall"
	"time"
)

// fillFromStat populates the platform-specific fields of fi from a Darwin
// syscall.Stat_t. On Darwin: Nlink is uint16, Dev and Rdev are int32,
// Blksize is int32, and the modification time is in Mtimespec.
// Implements prd002-sys R2.3 (Darwin side of the Darwin/Linux divergence).
func fillFromStat(fi *FileInfo, st *syscall.Stat_t) {
	fi.Nlink = uint64(st.Nlink)
	fi.Uid = st.Uid
	fi.Gid = st.Gid
	fi.Rdev = uint64(st.Rdev)
	fi.Dev = uint64(st.Dev)
	fi.Ino = st.Ino
	fi.Blocks = st.Blocks
	fi.Blksize = int64(st.Blksize)
	// R2.3: Darwin uses Mtimespec (not Mtim as on Linux).
	fi.ModTime = time.Unix(st.Mtimespec.Sec, st.Mtimespec.Nsec)
	fi.AccessTime = time.Unix(st.Atimespec.Sec, st.Atimespec.Nsec)
	fi.ChangeTime = time.Unix(st.Ctimespec.Sec, st.Ctimespec.Nsec)
}
