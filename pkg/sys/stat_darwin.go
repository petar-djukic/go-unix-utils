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

// fileInfoFromOS extracts FileInfo from os.FileInfo using the underlying
// Darwin-specific syscall.Stat_t.
// R1.4/R2.3: handles st_mtimespec/st_atimespec/st_ctimespec naming on Darwin.
func fileInfoFromOS(info os.FileInfo) (*FileInfo, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("sys: underlying Sys() is not *syscall.Stat_t")
	}
	return &FileInfo{
		Mode:       info.Mode(),
		Size:       stat.Size,
		Nlink:      uint64(stat.Nlink),
		Uid:        stat.Uid,
		Gid:        stat.Gid,
		Rdev:       uint64(stat.Rdev),
		Dev:        uint64(stat.Dev),
		Ino:        stat.Ino,
		Blocks:     stat.Blocks,
		Blksize:    int64(stat.Blksize),
		ModTime:    timespecToTime(stat.Mtimespec),
		AccessTime: timespecToTime(stat.Atimespec),
		ChangeTime: timespecToTime(stat.Ctimespec),
		Info:       info,
	}, nil
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}
