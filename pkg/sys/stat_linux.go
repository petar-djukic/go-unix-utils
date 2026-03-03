// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"syscall"
	"time"
)

// fillFromStat populates FileInfo fields from Linux's syscall.Stat_t.
// Linux uses Mtim (not Mtimespec) for modification time and has uint64 Dev
// and Rdev fields on amd64 (prd002-sys R2.3).
func fillFromStat(fi *FileInfo, st *syscall.Stat_t) {
	fi.Nlink = uint64(st.Nlink)
	fi.Uid = st.Uid
	fi.Gid = st.Gid
	fi.Rdev = uint64(st.Rdev)
	fi.Dev = uint64(st.Dev)
	fi.Ino = st.Ino
	fi.Blocks = st.Blocks
	fi.Blksize = int64(st.Blksize)
	fi.ModTime = time.Unix(st.Mtim.Sec, st.Mtim.Nsec)
}
