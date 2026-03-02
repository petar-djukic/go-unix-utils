// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"time"

	"golang.org/x/sys/unix"
)

// fillFromStat populates the platform-divergent fields of FileInfo from a
// Darwin unix.Stat_t. Darwin uses Mtimespec (not Mtim) for modification time,
// uint16 for Nlink, int32 for Dev/Rdev, and int32 for Blksize.
//
// prd002-sys R2.3.
func fillFromStat(fi *FileInfo, st *unix.Stat_t) {
	fi.ModTime = time.Unix(st.Mtimespec.Sec, st.Mtimespec.Nsec)
	fi.Nlink = uint64(st.Nlink)
	fi.Dev = uint64(st.Dev)
	fi.Rdev = uint64(st.Rdev)
	fi.Blksize = int64(st.Blksize)
}
