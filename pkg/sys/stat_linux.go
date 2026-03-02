// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"time"

	"golang.org/x/sys/unix"
)

// fillFromStat populates the platform-divergent fields of FileInfo from a
// Linux unix.Stat_t. Linux uses Mtim (not Mtimespec) for modification time.
// Field widths vary by architecture (e.g., Nlink is uint32 on arm64, uint64
// on amd64), so explicit conversions are used for portability.
//
// prd002-sys R2.3.
func fillFromStat(fi *FileInfo, st *unix.Stat_t) {
	fi.ModTime = time.Unix(st.Mtim.Sec, st.Mtim.Nsec)
	fi.Nlink = uint64(st.Nlink)
	fi.Dev = st.Dev
	fi.Rdev = st.Rdev
	fi.Blksize = int64(st.Blksize)
}
