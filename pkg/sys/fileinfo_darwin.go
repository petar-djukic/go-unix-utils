// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"syscall"
	"time"
)

// extractTimes populates ModTime, AccessTime, and ChangeTime from Darwin's
// Stat_t, which uses Mtimespec/Atimespec/Ctimespec field names.
// Implements prd002-sys R2.3.
func extractTimes(st *syscall.Stat_t, fi *FileInfo) {
	fi.ModTime = time.Unix(st.Mtimespec.Sec, st.Mtimespec.Nsec)
	fi.AccessTime = time.Unix(st.Atimespec.Sec, st.Atimespec.Nsec)
	fi.ChangeTime = time.Unix(st.Ctimespec.Sec, st.Ctimespec.Nsec)
}
