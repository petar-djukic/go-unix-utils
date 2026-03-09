// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"syscall"
	"time"
)

// fillTimes populates time fields from Linux's Stat_t (st_mtim, st_atim, st_ctim).
// R2.3: abstracts Darwin/Linux divergence in timestamp field names.
func fillTimes(fi *FileInfo, stat *syscall.Stat_t) {
	fi.ModTime = time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec)
	fi.AccessTime = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
	fi.ChangeTime = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
}
