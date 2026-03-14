// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps Darwin and Linux syscalls and signal handling.
// It provides a stable interface that cmd/ packages use to avoid
// platform-specific code in utility implementations.
//
// Implements: prd002-sys R1.1-R1.4
package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// FileInfo holds platform-normalized file metadata extracted from syscall.Stat_t.
// It abstracts Darwin vs Linux field name and type divergence, providing a single
// cross-platform representation for all cmd/ utilities that inspect file metadata.
//
// R1.1: FileInfo struct with all required fields.
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

// Stat returns a FileInfo for the file at path. Symbolic links are followed.
// All FileInfo fields are populated from the underlying syscall.Stat_t using
// golang.org/x/sys/unix for platform-safe field access.
//
// R1.2: Stat calls os.Stat and extracts syscall.Stat_t via os.FileInfo.Sys().
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %q: %w", path, err)
	}
	return buildFileInfo(info)
}

// Lstat returns a FileInfo for the file at path. Symbolic links are not followed;
// if path names a symlink, the returned FileInfo describes the link itself.
// All FileInfo fields are populated from the underlying syscall.Stat_t.
//
// R1.3: Lstat calls os.Lstat so symlinks are not followed.
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("lstat %q: %w", path, err)
	}
	return buildFileInfo(info)
}

// buildFileInfo converts an os.FileInfo into a FileInfo by extracting the
// underlying syscall.Stat_t. Fields with identical types on Darwin and Linux
// (Uid, Gid, Ino, Blocks) are set here. Platform-divergent fields (Nlink, Dev,
// Rdev, Blksize, and timestamp field names) are delegated to setStatFields.
func buildFileInfo(info os.FileInfo) (*FileInfo, error) {
	raw, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("pkg/sys: Sys() did not return *syscall.Stat_t on this platform")
	}
	fi := &FileInfo{
		Mode:   info.Mode(),
		Size:   info.Size(),
		Uid:    raw.Uid,
		Gid:    raw.Gid,
		Ino:    raw.Ino,
		Blocks: raw.Blocks,
		Info:   info,
	}
	// R1.4: Platform-divergent fields are set by setStatFields (Darwin vs Linux).
	setStatFields(fi, raw)
	return fi, nil
}
