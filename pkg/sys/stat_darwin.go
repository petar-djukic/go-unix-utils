// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

// stat_darwin.go implements Darwin-specific file metadata extraction.
// On Darwin, the modification time field is Mtimespec and st_blocks
// are in 512-byte units.
package sys

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

func extractMetadata(info os.FileInfo) (FileMetadata, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return FileMetadata{}, fmt.Errorf("extracting metadata: expected *syscall.Stat_t, got %T", info.Sys())
	}
	return FileMetadata{
		ModTime: time.Unix(stat.Mtimespec.Sec, stat.Mtimespec.Nsec),
		Blocks:  stat.Blocks,
	}, nil
}
