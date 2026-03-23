// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// fileInfoFromOS populates a FileInfo from os.FileInfo using Linux's
// syscall.Stat_t layout.
//
// R2.3: handles Linux-specific field names (Mtim, Atim, Ctim).
func fileInfoFromOS(info os.FileInfo) (*FileInfo, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("sys.Stat: unexpected Sys() type for %s", info.Name())
	}
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
		Blksize:    st.Blksize,
		ModTime:    timespecToTime(st.Mtim),
		AccessTime: timespecToTime(st.Atim),
		ChangeTime: timespecToTime(st.Ctim),
		Info:       info,
	}, nil
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}
