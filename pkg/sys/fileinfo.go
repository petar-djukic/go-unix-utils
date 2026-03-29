// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// FileInfo holds extended file metadata populated from syscall.Stat_t.
//
// R1.1 (prd002): fields cover mode, size, link count, ownership, device,
// inode, block counts, and timestamps including access and change times.
// R2.2: struct layout matches the package contract.
type FileInfo struct {
	Mode       os.FileMode
	Size       int64
	Nlink      uint64
	Uid        uint32
	Gid        uint32
	Rdev       uint64
	Dev        uint64
	Ino        uint64
	Blocks     int64
	Blksize    int64
	ModTime    time.Time
	AccessTime time.Time
	ChangeTime time.Time
	Info       os.FileInfo
}

// Stat returns extended file metadata for path, following symbolic links.
//
// R1.5 (prd002): calls os.Stat and populates FileInfo from syscall.Stat_t.
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return populateFileInfo(info)
}

// Lstat returns extended file metadata for path without following symbolic links.
//
// R1.6 (prd002): calls os.Lstat and populates FileInfo from syscall.Stat_t.
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	return populateFileInfo(info)
}

// populateFileInfo extracts syscall.Stat_t from os.FileInfo and builds a FileInfo.
func populateFileInfo(info os.FileInfo) (*FileInfo, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("sys: unsupported platform Sys() type %T", info.Sys())
	}
	fi := &FileInfo{
		Mode:    info.Mode(),
		Size:    info.Size(),
		Uid:     st.Uid,
		Gid:     st.Gid,
		ModTime: info.ModTime(),
		Info:    info,
	}
	populatePlatformFields(fi, st)
	return fi, nil
}
