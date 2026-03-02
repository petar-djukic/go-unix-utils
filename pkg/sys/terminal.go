// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd002-sys (R1)
package sys

import (
	"fmt"
	"syscall"
	"unsafe"
)

// WinSize holds terminal dimensions as reported by the TIOCGWINSZ ioctl.
// The struct layout matches C's struct winsize (rows and cols fields).
// (prd002-sys R1)
type WinSize struct {
	Rows uint16 // terminal row count
	Cols uint16 // terminal column count
}

// IsTerminal reports whether the file descriptor refers to a terminal.
// Detection uses the TIOCGWINSZ ioctl: a successful call indicates a terminal;
// any error indicates a non-terminal fd (pipe, regular file, etc.).
// (prd002-sys R1.3)
func IsTerminal(fd uintptr) bool {
	var ws [8]byte // struct winsize: 4 x uint16 = 8 bytes
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws[0])))
	return errno == 0
}

// GetWinSize returns the terminal dimensions (rows and columns) for the given
// file descriptor by reading the TIOCGWINSZ ioctl result. Returns an error if
// the fd is not a terminal.
// (prd002-sys R1)
func GetWinSize(fd uintptr) (WinSize, error) {
	// C struct winsize: rows (uint16), cols (uint16), xpixel (uint16), ypixel (uint16).
	type winsize struct {
		rows   uint16
		cols   uint16
		xpixel uint16
		ypixel uint16
	}
	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	if errno != 0 {
		return WinSize{}, fmt.Errorf("get terminal size: %w", errno)
	}
	return WinSize{Rows: ws.rows, Cols: ws.cols}, nil
}
