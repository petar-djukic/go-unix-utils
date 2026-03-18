// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"syscall"
	"time"
)

// extractTimes populates ModTime, AccessTime, and ChangeTime from Linux's
// Stat_t, which uses Mtim/Atim/Ctim field names.
// Implements prd002-sys R2.3.
func extractTimes(st *syscall.Stat_t, fi *FileInfo) {
	fi.ModTime = time.Unix(st.Mtim.Sec, st.Mtim.Nsec)
	fi.AccessTime = time.Unix(st.Atim.Sec, st.Atim.Nsec)
	fi.ChangeTime = time.Unix(st.Ctim.Sec, st.Ctim.Nsec)
}
