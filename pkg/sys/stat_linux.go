// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"syscall"
	"time"
)

// populatePlatformFields sets platform-specific FileInfo fields from Linux's Stat_t.
//
// R1.7 (prd002): Linux uses Mtim, Atim, Ctim (not Mtimespec, Atimespec, Ctimespec).
func populatePlatformFields(fi *FileInfo, st *syscall.Stat_t) {
	fi.Nlink = st.Nlink
	fi.Rdev = st.Rdev
	fi.Dev = st.Dev
	fi.Ino = st.Ino
	fi.Blocks = st.Blocks
	fi.Blksize = st.Blksize
	fi.AccessTime = timespecToTime(st.Atim)
	fi.ChangeTime = timespecToTime(st.Ctim)
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}
