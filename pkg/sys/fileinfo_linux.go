// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// fileInfoFromStat populates FileInfo from a Linux unix.Stat_t.
// R2.3: abstracts Linux st_mtim (vs Darwin st_mtimespec) and Nlink type (uint64 on Linux).
func fileInfoFromStat(st *unix.Stat_t, info os.FileInfo) *FileInfo {
	return &FileInfo{
		Mode:       info.Mode(),
		Size:       st.Size,
		Nlink:      st.Nlink,
		Uid:        st.Uid,
		Gid:        st.Gid,
		Rdev:       st.Rdev,
		Dev:        st.Dev,
		Ino:        st.Ino,
		Blocks:     st.Blocks,
		Blksize:    int64(st.Blksize),
		ModTime:    time.Unix(st.Mtim.Sec, st.Mtim.Nsec),
		AccessTime: time.Unix(st.Atim.Sec, st.Atim.Nsec),
		ChangeTime: time.Unix(st.Ctim.Sec, st.Ctim.Nsec),
		Info:       info,
	}
}
