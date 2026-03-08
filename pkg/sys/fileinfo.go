// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps Darwin and Linux syscalls and signal handling.
//
// Implements prd002-sys: FileInfo, Stat, Lstat, TerminalWidth, IsTerminal,
// InstallSIGPIPEHandler, OnTerminalResize.
package sys

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// FileInfo holds extended file metadata from syscall.Stat_t. R2.2.
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

// Stat returns extended file metadata for path, following symlinks. R2.1.
func Stat(path string) (*FileInfo, error) {
	var stat unix.Stat_t
	if err := unix.Stat(path, &stat); err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	return fromStat(&stat, info), nil
}

// Lstat returns extended file metadata for path without following symlinks. R2.1.
func Lstat(path string) (*FileInfo, error) {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}

	return fromStat(&stat, info), nil
}

// unixModeToFileMode converts a raw Unix mode (uint32) to os.FileMode.
func unixModeToFileMode(mode uint32) os.FileMode {
	var fm os.FileMode

	// Permission bits.
	fm = os.FileMode(mode & 0o777)

	// Setuid, setgid, sticky.
	if mode&unix.S_ISUID != 0 {
		fm |= os.ModeSetuid
	}
	if mode&unix.S_ISGID != 0 {
		fm |= os.ModeSetgid
	}
	if mode&unix.S_ISVTX != 0 {
		fm |= os.ModeSticky
	}

	// File type bits.
	switch mode & unix.S_IFMT {
	case unix.S_IFDIR:
		fm |= os.ModeDir
	case unix.S_IFLNK:
		fm |= os.ModeSymlink
	case unix.S_IFBLK:
		fm |= os.ModeDevice
	case unix.S_IFCHR:
		fm |= os.ModeDevice | os.ModeCharDevice
	case unix.S_IFIFO:
		fm |= os.ModeNamedPipe
	case unix.S_IFSOCK:
		fm |= os.ModeSocket
	}

	return fm
}
