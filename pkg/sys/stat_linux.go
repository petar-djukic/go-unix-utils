// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"syscall"
	"time"
)

// fillTimes populates ModTime, AccessTime, and ChangeTime from Linux's
// Mtim, Atim, and Ctim fields. R2.3.
func fillTimes(fi *FileInfo, stat *syscall.Stat_t) {
	fi.ModTime = time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec)
	fi.AccessTime = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
	fi.ChangeTime = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
}
