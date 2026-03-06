// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"syscall"
	"time"
)

// fillFromStat populates the platform-specific fields of fi from a Linux
// syscall.Stat_t. On Linux: Nlink, Dev, and Rdev are uint64, Blksize is
// int64, and the modification time is in Mtim.
// Implements prd002-sys R2.3 (Linux side of the Darwin/Linux divergence).
func fillFromStat(fi *FileInfo, st *syscall.Stat_t) {
	// Explicit conversions handle field-size differences across Linux architectures.
	// On linux/amd64: Nlink uint64, Blksize int64.
	// On linux/arm64: Nlink uint32, Blksize int32.
	fi.Nlink = uint64(st.Nlink)
	fi.Uid = st.Uid
	fi.Gid = st.Gid
	fi.Rdev = uint64(st.Rdev)
	fi.Dev = uint64(st.Dev)
	fi.Ino = st.Ino
	fi.Blocks = st.Blocks
	fi.Blksize = int64(st.Blksize)
	// R2.3: Linux uses Mtim (not Mtimespec as on Darwin).
	fi.ModTime = time.Unix(st.Mtim.Sec, st.Mtim.Nsec)
	fi.AccessTime = time.Unix(st.Atim.Sec, st.Atim.Nsec)
	fi.ChangeTime = time.Unix(st.Ctim.Sec, st.Ctim.Nsec)
}
