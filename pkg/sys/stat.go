// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd002-sys (R2)
package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// FileMetadata holds platform-independent file metadata extracted from
// syscall.Stat_t. It abstracts the Darwin/Linux field-name divergence so
// cmd/ packages never import platform-specific syscall types.
// (prd002-sys R2.1, R2.2, R2.3)
type FileMetadata struct {
	ModTime time.Time // modification time (st_mtime / st_mtimespec)
	ATime   time.Time // access time (st_atime / st_atimespec)
	Blocks  int64     // number of 512-byte blocks allocated (st_blocks)
	Blksize int64     // preferred I/O block size (st_blksize)
	Dev     uint64    // device ID of containing filesystem (st_dev)
	Ino     uint64    // inode number (st_ino)
	Nlink   uint64    // hard-link count (st_nlink)
	Uid     uint32    // owner user ID (st_uid)
	Gid     uint32    // owner group ID (st_gid)
}

// Stat returns file metadata for path, following symbolic links.
// It calls os.Stat and extracts the underlying syscall.Stat_t to populate
// FileMetadata via the platform-specific extractMetadata function.
// (prd002-sys R2.1)
func Stat(path string) (FileMetadata, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return FileMetadata{}, fmt.Errorf("stat %s: %w", path, err)
	}
	return metadataFromFileInfo(fi)
}

// Lstat returns file metadata for path without following symbolic links.
// It calls os.Lstat and extracts the underlying syscall.Stat_t to populate
// FileMetadata via the platform-specific extractMetadata function.
// (prd002-sys R2.1)
func Lstat(path string) (FileMetadata, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return FileMetadata{}, fmt.Errorf("lstat %s: %w", path, err)
	}
	return metadataFromFileInfo(fi)
}

// metadataFromFileInfo extracts the underlying *syscall.Stat_t from an
// os.FileInfo and delegates to the platform-specific extractMetadata function.
func metadataFromFileInfo(fi os.FileInfo) (FileMetadata, error) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return FileMetadata{}, fmt.Errorf("unsupported FileInfo.Sys() type %T", fi.Sys())
	}
	return extractMetadata(st), nil
}
