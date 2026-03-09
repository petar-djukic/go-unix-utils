// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys provides syscall abstractions for Darwin and Linux,
// wrapping terminal queries, extended file metadata, and signal handling.
// Implements prd002-sys (R1, R2, R3).
package sys

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// FileInfo holds extended file metadata from stat(2), abstracting
// Darwin/Linux divergence in struct field names (st_mtimespec vs st_mtim,
// st_blocks interpretation). Implements prd002-sys R2.2.
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
	AccessTime time.Time   // access time (st_atime / st_atimespec)
	ChangeTime time.Time   // status change time (st_ctime / st_ctimespec)
	Info       os.FileInfo // underlying os.FileInfo for os package compatibility
}

// Stat returns extended file metadata for path, following symbolic links.
// Equivalent to os.Stat but populates all FileInfo fields from unix.Stat_t.
// Implements prd002-sys R2.1.
func Stat(path string) (*FileInfo, error) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	fi := fillFromStat(&st, info)
	return fi, nil
}

// Lstat returns extended file metadata for path without following symbolic
// links. Equivalent to os.Lstat but populates all FileInfo fields from
// unix.Stat_t. Implements prd002-sys R2.1.
func Lstat(path string) (*FileInfo, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}

	fi := fillFromStat(&st, info)
	return fi, nil
}

// fillFromStat populates a FileInfo from a unix.Stat_t and os.FileInfo.
// golang.org/x/sys/unix normalizes the Darwin/Linux time field names
// (Mtim/Atim/Ctim on both platforms), so no build-tagged files are needed.
// Implements prd002-sys R2.3.
func fillFromStat(st *unix.Stat_t, info os.FileInfo) *FileInfo {
	fi := &FileInfo{
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
	return fi
}
