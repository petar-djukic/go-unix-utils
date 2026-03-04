// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin || linux

// Package sys wraps Darwin and Linux syscalls and signal handling. It provides
// a stable interface that cmd/ packages use to avoid platform-specific code in
// utility implementations.
//
// Implements prd002-sys (R2).
package sys

import (
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

// FileMetadata provides platform-independent access to file metadata fields
// that diverge between Darwin and Linux (modification time from
// st_mtimespec/st_mtim, disk block count from st_blocks).
// R2.2, R2.3: abstracts Darwin/Linux stat struct divergence.
type FileMetadata struct {
	modTime time.Time
	blocks  int64
}

// ModTime returns the file's modification time extracted from the underlying
// platform-specific stat struct (st_mtimespec on Darwin, st_mtim on Linux).
// R2.2: modification time accessor.
func (m *FileMetadata) ModTime() time.Time {
	return m.modTime
}

// Blocks returns the number of 512-byte blocks allocated for the file,
// extracted from st_blocks in the underlying stat struct.
// R2.2: disk block count accessor.
func (m *FileMetadata) Blocks() int64 {
	return m.blocks
}

// Stat returns file metadata for the named file, following symbolic links.
// Equivalent to os.Stat but populates platform-specific fields via
// golang.org/x/sys/unix.
// R2.1: stat with symlink resolution.
func Stat(path string) (*FileMetadata, error) {
	var stat unix.Stat_t
	if err := unix.Stat(path, &stat); err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	return newFileMetadata(&stat), nil
}

// Lstat returns file metadata for the named file without following symbolic
// links. Equivalent to os.Lstat but populates platform-specific fields via
// golang.org/x/sys/unix.
// R2.1: lstat without symlink resolution.
func Lstat(path string) (*FileMetadata, error) {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return nil, fmt.Errorf("lstat %s: %w", path, err)
	}
	return newFileMetadata(&stat), nil
}
