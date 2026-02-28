// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

// stat_linux.go implements Linux-specific file metadata extraction.
// On Linux, the modification time field is Mtim and st_blocks
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
		ModTime: time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec),
		Blocks:  stat.Blocks,
	}, nil
}
