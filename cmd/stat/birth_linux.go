// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build linux

package main

import (
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// birthTime returns false on Linux where birth time is not available
// via syscall.Stat_t.
func birthTime(_ *sys.FileInfo) (time.Time, bool) {
	return time.Time{}, false
}
