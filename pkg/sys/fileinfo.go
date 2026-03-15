// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R2.1-R2.3: FileInfo struct, Stat, and Lstat functions.

package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// FileInfo extends os.FileInfo with fields from syscall.Stat_t that Go's
// standard library does not expose: hard-link count, owner IDs, device and
// inode numbers, block counts, and all three stat timestamps.
//
// R2.2: named struct with all fields from the package contract.
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

// Stat calls os.Stat on path and returns a *FileInfo populated from the
// underlying syscall.Stat_t. Stat follows symbolic links.
//
// R2.1: follows symlinks via os.Stat.
func Stat(path string) (*FileInfo, error) {
	return statPath(path, os.Stat)
}

// Lstat calls os.Lstat on path and returns a *FileInfo populated from the
// underlying syscall.Stat_t. Lstat does not follow symbolic links.
//
// R2.1: does not follow symlinks via os.Lstat.
func Lstat(path string) (*FileInfo, error) {
	return statPath(path, os.Lstat)
}

// statPath is the shared implementation for Stat and Lstat. The statFn
// parameter selects between os.Stat and os.Lstat.
func statPath(path string, statFn func(string) (os.FileInfo, error)) (*FileInfo, error) {
	info, err := statFn(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("stat %s: underlying Sys() is not *syscall.Stat_t", path)
	}

	fi := &FileInfo{
		Mode: info.Mode(),
		Size: stat.Size,
		Uid:  stat.Uid,
		Gid:  stat.Gid,
		Info: info,
	}

	// R2.3: delegate platform-divergent field extraction to build-tagged helper.
	populateFromStat(fi, stat)

	return fi, nil
}
