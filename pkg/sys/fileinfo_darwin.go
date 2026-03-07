// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"time"

	"golang.org/x/sys/unix"
)

// fillFromStat populates a FileInfo from a Darwin unix.Stat_t.
// Darwin has int32 Dev/Rdev, uint16 Nlink, and int32 Blksize which require
// widening casts to match the FileInfo contract. (prd002-sys R2.3)
func fillFromStat(st *unix.Stat_t) *FileInfo {
	return &FileInfo{
		Size:       st.Size,
		Nlink:      uint64(st.Nlink),
		Uid:        st.Uid,
		Gid:        st.Gid,
		Rdev:       uint64(uint32(st.Rdev)),
		Dev:        uint64(uint32(st.Dev)),
		Ino:        st.Ino,
		Blocks:     st.Blocks,
		Blksize:    int64(st.Blksize),
		ModTime:    time.Unix(st.Mtim.Sec, st.Mtim.Nsec),
		AccessTime: time.Unix(st.Atim.Sec, st.Atim.Nsec),
		ChangeTime: time.Unix(st.Ctim.Sec, st.Ctim.Nsec),
	}
}
