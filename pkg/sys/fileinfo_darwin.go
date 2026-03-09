// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"syscall"
	"time"
)

// fillTimes populates time fields from Darwin's Stat_t (st_mtimespec, st_atimespec, st_ctimespec).
// R2.3: abstracts Darwin/Linux divergence in timestamp field names.
func fillTimes(fi *FileInfo, stat *syscall.Stat_t) {
	fi.ModTime = time.Unix(stat.Mtimespec.Sec, stat.Mtimespec.Nsec)
	fi.AccessTime = time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec)
	fi.ChangeTime = time.Unix(stat.Ctimespec.Sec, stat.Ctimespec.Nsec)
}
