// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"time"

	"golang.org/x/sys/unix"
)

// newFileMetadata extracts platform-independent metadata from a Linux
// unix.Stat_t. Reads Mtim (Linux's st_mtim) for modification time and Blocks
// for 512-byte block count.
// R2.3: Linux-specific stat field extraction via golang.org/x/sys/unix.
func newFileMetadata(stat *unix.Stat_t) *FileMetadata {
	return &FileMetadata{
		modTime: time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec),
		blocks:  stat.Blocks,
	}
}
