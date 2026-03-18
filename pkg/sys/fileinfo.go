// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys provides portable syscall abstractions for Darwin and Linux.
// Implements prd002-sys R2.1–R2.3: FileInfo struct, Stat, and Lstat.
package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// FileInfo holds extended file metadata from syscall.Stat_t fields not
// exposed by os.FileInfo. Implements prd002-sys R2.2.
type FileInfo struct {
	Mode       os.FileMode // file mode and type bits (from syscall.Stat_t.Mode)
	Size       int64       // apparent file size in bytes (st_size)
	Nlink      uint64      // hard-link count (st_nlink)
	Uid        uint32      // owner user ID (st_uid)
	Gid        uint32      // owner group ID (st_gid)
	Rdev       uint64      // device ID for special files (st_rdev)
	Dev        uint64      // device ID of the containing filesystem (st_dev)
	Ino        uint64      // inode number (st_ino)
	Blocks     int64       // number of 512-byte blocks allocated (st_blocks)
	Blksize    int64       // preferred I/O block size (st_blksize)
	ModTime    time.Time   // modification time (st_mtime / st_mtimespec)
	AccessTime time.Time   // last access time (st_atime / st_atimespec)
	ChangeTime time.Time   // status change time (st_ctime / st_ctimespec)
	Info       os.FileInfo // underlying os.FileInfo for os package compatibility
}

// Stat returns a *FileInfo for the named file, following symbolic links.
// Implements prd002-sys R2.1.
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	return fileInfoFromOS(info)
}

// Lstat returns a *FileInfo for the named file without following symbolic
// links. Implements prd002-sys R2.1.
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}
	return fileInfoFromOS(info)
}

// fileInfoFromOS extracts FileInfo fields from the os.FileInfo's underlying
// syscall.Stat_t. Platform-specific time extraction is delegated to
// extractTimes (defined in fileinfo_darwin.go / fileinfo_linux.go).
func fileInfoFromOS(info os.FileInfo) (*FileInfo, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("unexpected Sys() type %T", info.Sys())
	}
	fi := &FileInfo{
		Mode:    info.Mode(),
		Size:    st.Size,
		Nlink:   uint64(st.Nlink),
		Uid:     st.Uid,
		Gid:     st.Gid,
		Rdev:    uint64(st.Rdev),
		Dev:     uint64(st.Dev),
		Ino:     uint64(st.Ino),
		Blocks:  st.Blocks,
		Blksize: int64(st.Blksize),
		Info:    info,
	}
	extractTimes(st, fi)
	return fi, nil
}
