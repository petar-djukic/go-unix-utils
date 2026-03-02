// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// FileInfo holds extended file metadata from the underlying unix.Stat_t,
// exposing fields not available from os.FileInfo: hard-link count, device
// and inode numbers, physical block counts, and nanosecond-precision
// modification time. The struct abstracts Darwin/Linux divergence in Stat_t
// field names and types so callers use a single API regardless of platform.
//
// prd002-sys R2.2.
type FileInfo struct {
	Mode    os.FileMode // file mode and type bits (from syscall.Stat_t.Mode)
	Size    int64       // apparent file size in bytes (st_size)
	Nlink   uint64      // hard-link count (st_nlink)
	Uid     uint32      // owner user ID (st_uid)
	Gid     uint32      // owner group ID (st_gid)
	Rdev    uint64      // device ID for special files (st_rdev)
	Dev     uint64      // device ID of the containing filesystem (st_dev)
	Ino     uint64      // inode number (st_ino)
	Blocks  int64       // number of 512-byte blocks allocated (st_blocks)
	Blksize int64       // preferred I/O block size (st_blksize)
	ModTime time.Time   // modification time (st_mtime / st_mtimespec)
	Info    os.FileInfo // underlying os.FileInfo for os package compatibility
}

// Stat returns extended file metadata for path, following symbolic links.
// It combines os.Stat (for os.FileInfo compatibility) with unix.Stat (for
// extended Stat_t fields). Platform-divergent field extraction is handled
// by fillFromStat in stat_darwin.go / stat_linux.go.
//
// prd002-sys R2.1.
func Stat(path string) (*FileInfo, error) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	osInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	return newFileInfo(&st, osInfo), nil
}

// Lstat returns extended file metadata for path without following symbolic
// links. When path is a symlink, the returned FileInfo describes the symlink
// itself, not its target.
//
// prd002-sys R2.1.
func Lstat(path string) (*FileInfo, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}
	osInfo, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}
	return newFileInfo(&st, osInfo), nil
}

// newFileInfo populates a FileInfo from a unix.Stat_t and the corresponding
// os.FileInfo. Fields with identical types across Darwin and Linux are set
// here. Platform-divergent fields (ModTime, Nlink, Dev, Rdev, Blksize) are
// populated by fillFromStat, defined in stat_darwin.go and stat_linux.go.
//
// prd002-sys R2.2, R2.3.
func newFileInfo(st *unix.Stat_t, osInfo os.FileInfo) *FileInfo {
	fi := &FileInfo{
		Mode:   osInfo.Mode(),
		Size:   st.Size,
		Uid:    st.Uid,
		Gid:    st.Gid,
		Ino:    st.Ino,
		Blocks: st.Blocks,
		Info:   osInfo,
	}
	fillFromStat(fi, st)
	return fi
}
