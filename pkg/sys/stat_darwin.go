// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"syscall"
	"time"
)

// populatePlatformFields sets platform-specific FileInfo fields from Darwin's Stat_t.
//
// R1.7 (prd002): Darwin uses Mtimespec, Atimespec, Ctimespec (not Mtim, Atim, Ctim).
func populatePlatformFields(fi *FileInfo, st *syscall.Stat_t) {
	fi.Nlink = uint64(st.Nlink)
	fi.Rdev = uint64(st.Rdev)
	fi.Dev = uint64(st.Dev)
	fi.Ino = st.Ino
	fi.Blocks = st.Blocks
	fi.Blksize = int64(st.Blksize)
	fi.AccessTime = timespecToTime(st.Atimespec)
	fi.ChangeTime = timespecToTime(st.Ctimespec)
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}
