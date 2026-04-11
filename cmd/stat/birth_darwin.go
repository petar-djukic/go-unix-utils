// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin

package main

import (
	"syscall"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// birthTime extracts the birth time from Darwin's Birthtimespec.
// R3.1: %w and %W directives require birth time access.
func birthTime(fi *sys.FileInfo) (time.Time, bool) {
	stat, ok := fi.Info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, false
	}
	ts := stat.Birthtimespec
	if ts.Sec == 0 && ts.Nsec == 0 {
		return time.Time{}, false
	}
	return time.Unix(ts.Sec, ts.Nsec), true
}
