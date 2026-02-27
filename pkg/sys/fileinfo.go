// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// FileInfo contains extended file metadata from syscall.Stat_t that is not
// available through os.FileInfo. The struct abstracts Darwin/Linux field name
// and type divergence so callers use a single API.
//
// Per prd002-sys R2.2.
type FileInfo struct {
	// Mode is the file mode including type bits and permission bits.
	Mode os.FileMode

	// Size is the file size in bytes.
	Size int64

	// Nlink is the number of hard links (st_nlink).
	Nlink uint64

	// Uid is the user ID of the file owner (st_uid).
	Uid uint32

	// Gid is the group ID of the file owner (st_gid).
	Gid uint32

	// Rdev is the device ID for special files (st_rdev).
	Rdev uint64

	// Dev is the ID of the device containing the file (st_dev).
	Dev uint64

	// Ino is the inode number (st_ino).
	Ino uint64

	// Blocks is the number of 512-byte blocks allocated (st_blocks).
	// On both Darwin and Linux, st_blocks counts in 512-byte units.
	Blocks int64

	// Blksize is the preferred I/O block size (st_blksize).
	Blksize int64

	// ModTime is the file modification time derived from st_mtimespec (Darwin)
	// or st_mtim (Linux).
	ModTime time.Time

	// Info is the underlying os.FileInfo for compatibility with os package
	// functions.
	Info os.FileInfo
}

// Stat returns extended file metadata for path, following symbolic links.
//
// Per prd002-sys R2.1.
// Utility context: ls needs st_nlink, st_uid, st_gid, st_rdev for -l format;
// du needs st_dev and st_ino for hard-link deduplication (du.c:554).
func Stat(path string) (*FileInfo, error) {
	return statPath(path, true)
}

// Lstat returns extended file metadata for path without following symbolic
// links. When path is a symlink, the returned FileInfo describes the symlink
// itself (type bits show symlink), not its target.
//
// Per prd002-sys R2.1, AC3.
func Lstat(path string) (*FileInfo, error) {
	return statPath(path, false)
}

// statPath is the shared implementation for Stat and Lstat. When followLinks
// is true, it calls os.Stat; otherwise os.Lstat.
func statPath(path string, followLinks bool) (*FileInfo, error) {
	var info os.FileInfo
	var err error
	if followLinks {
		info, err = os.Stat(path)
	} else {
		info, err = os.Lstat(path)
	}
	if err != nil {
		return nil, err
	}

	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("extracting stat fields from %s: unsupported platform", path)
	}

	fi := &FileInfo{
		Mode: info.Mode(),
		Size: info.Size(),
		Info: info,
	}
	// fillFromStat is defined in stat_darwin.go and stat_linux.go.
	// It abstracts the Darwin/Linux divergence per prd002-sys R2.3.
	fillFromStat(fi, st)

	return fi, nil
}
