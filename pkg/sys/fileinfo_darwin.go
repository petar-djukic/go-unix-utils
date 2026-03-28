// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// fileInfoFromOS populates a FileInfo from os.FileInfo using Darwin's
// syscall.Stat_t layout.
//
// R2.3: handles Darwin-specific field names (Mtimespec, Atimespec, Ctimespec)
// and type widths (Dev/Rdev are int32, Nlink is uint16, Blksize is int32).
func fileInfoFromOS(info os.FileInfo) (*FileInfo, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("sys.Stat: unexpected Sys() type for %s", info.Name())
	}
	return &FileInfo{
		Mode:       info.Mode(),
		Size:       st.Size,
		Nlink:      uint64(st.Nlink),
		Uid:        st.Uid,
		Gid:        st.Gid,
		Rdev:       uint64(uint32(st.Rdev)),
		Dev:        uint64(uint32(st.Dev)),
		Ino:        st.Ino,
		Blocks:     st.Blocks,
		Blksize:    int64(st.Blksize),
		ModTime:    timespecToTime(st.Mtimespec),
		AccessTime: timespecToTime(st.Atimespec),
		ChangeTime: timespecToTime(st.Ctimespec),
		Info:       info,
	}, nil
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}
