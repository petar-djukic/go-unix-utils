// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"time"

	"golang.org/x/sys/unix"
)

// fillFromStat populates a FileInfo from a Linux unix.Stat_t.
// Linux uses Mtim/Atim/Ctim field names and has uint64 Dev/Rdev/Nlink
// and int64 Blksize on amd64. (prd002-sys R2.3)
func fillFromStat(st *unix.Stat_t) *FileInfo {
	return &FileInfo{
		Size:       st.Size,
		Nlink:      st.Nlink,
		Uid:        st.Uid,
		Gid:        st.Gid,
		Rdev:       st.Rdev,
		Dev:        st.Dev,
		Ino:        st.Ino,
		Blocks:     st.Blocks,
		Blksize:    st.Blksize,
		ModTime:    time.Unix(st.Mtim.Sec, st.Mtim.Nsec),
		AccessTime: time.Unix(st.Atim.Sec, st.Atim.Nsec),
		ChangeTime: time.Unix(st.Ctim.Sec, st.Ctim.Nsec),
	}
}
