// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// fileInfoFromStat populates FileInfo from a Darwin unix.Stat_t.
// R2.3: abstracts Darwin-specific types (int32 Dev/Rdev, uint16 Nlink).
func fileInfoFromStat(st *unix.Stat_t, info os.FileInfo) *FileInfo {
	return &FileInfo{
		Mode:       info.Mode(),
		Size:       st.Size,
		Nlink:      uint64(st.Nlink),
		Uid:        st.Uid,
		Gid:        st.Gid,
		Rdev:       uint64(st.Rdev),
		Dev:        uint64(st.Dev),
		Ino:        st.Ino,
		Blocks:     st.Blocks,
		Blksize:    int64(st.Blksize),
		ModTime:    time.Unix(st.Mtim.Sec, st.Mtim.Nsec),
		AccessTime: time.Unix(st.Atim.Sec, st.Atim.Nsec),
		ChangeTime: time.Unix(st.Ctim.Sec, st.Ctim.Nsec),
		Info:       info,
	}
}
