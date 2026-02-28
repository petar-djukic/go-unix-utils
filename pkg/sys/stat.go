// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys wraps Darwin and Linux syscalls and signal handling, providing
// a stable interface that cmd/ packages use to avoid platform-specific code
// in utility implementations.
//
// Implements: prd002-sys R1, R2, R3, R4.
package sys

import (
	"os"
	"time"
)

// FileMetadata contains platform-independent file metadata extracted from
// the underlying syscall.Stat_t that os.FileInfo does not directly expose.
//
// Implements: prd002-sys R2.
type FileMetadata struct {
	// ModTime is the file modification time with nanosecond precision,
	// extracted from st_mtimespec (Darwin) or st_mtim (Linux).
	ModTime time.Time

	// Blocks is the number of 512-byte blocks allocated on disk.
	Blocks int64
}

// ExtractMetadata extracts platform-specific file metadata from an os.FileInfo.
// The underlying Sys() value must be a *syscall.Stat_t; returns an error if
// the type assertion fails.
//
// Implements: prd002-sys R2.3.
func ExtractMetadata(info os.FileInfo) (FileMetadata, error) {
	return extractMetadata(info)
}
