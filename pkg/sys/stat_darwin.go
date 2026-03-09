// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"syscall"
	"time"
)

// fillTimes populates ModTime, AccessTime, and ChangeTime from Darwin's
// Mtimespec, Atimespec, and Ctimespec fields. R2.3.
func fillTimes(fi *FileInfo, stat *syscall.Stat_t) {
	fi.ModTime = time.Unix(stat.Mtimespec.Sec, stat.Mtimespec.Nsec)
	fi.AccessTime = time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec)
	fi.ChangeTime = time.Unix(stat.Ctimespec.Sec, stat.Ctimespec.Nsec)
}
