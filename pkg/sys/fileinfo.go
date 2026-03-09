// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys provides portable syscall abstractions for go-unix-utils.
// It wraps platform-divergent stat fields, terminal queries, and signal
// handling into a single API that cmd/ packages consume without importing
// syscall or golang.org/x/sys directly.
//
// Implements: prd002-sys (R1–R4).
package sys

import (
	"os"
	"time"
)

// FileInfo wraps platform-divergent stat fields into a single portable struct.
// R2.2: all fields populated from syscall.Stat_t.
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
