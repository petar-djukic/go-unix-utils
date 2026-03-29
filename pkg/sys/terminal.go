// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"unsafe"
)

// winsize mirrors the C struct winsize used by TIOCGWINSZ.
type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// stdoutFD is the file descriptor for stdout, used by TerminalWidth.
// R2.2 (prd002): query terminal width on stdout fd.
const stdoutFD = 1

// TerminalWidth returns the current terminal column count by calling
// ioctl(TIOCGWINSZ) on stdout.
//
// R2.2 (prd002): returns an error when stdout is not a terminal or ioctl fails.
func TerminalWidth() (int, error) {
	w, err := ioctlGetWinsize(stdoutFD)
	if err != nil {
		return 0, fmt.Errorf("sys: terminal width: %w", err)
	}
	return int(w.Col), nil
}

// IsTerminal returns true when the file descriptor refers to a terminal.
//
// R2.3 (prd002): attempts TIOCGWINSZ ioctl; success means fd is a terminal.
func IsTerminal(fd uintptr) bool {
	_, err := ioctlGetWinsize(int(fd))
	return err == nil
}

// ioctlGetWinsize performs the TIOCGWINSZ ioctl on the given fd.
func ioctlGetWinsize(fd int) (*winsize, error) {
	var ws winsize
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 {
		return nil, errno
	}
	return &ws, nil
}

// resizeMu protects the resizeCallbacks slice.
var resizeMu sync.Mutex

// resizeCallbacks holds registered SIGWINCH callbacks.
var resizeCallbacks []func(width int)

// resizeOnce ensures the SIGWINCH listener goroutine starts only once.
var resizeOnce sync.Once

// OnTerminalResize registers a callback invoked with the new terminal width
// when SIGWINCH is received.
//
// R2.5 (prd002): supports multiple registrations; callbacks are called in order.
func OnTerminalResize(callback func(width int)) {
	resizeMu.Lock()
	resizeCallbacks = append(resizeCallbacks, callback)
	resizeMu.Unlock()

	resizeOnce.Do(startResizeListener)
}

// startResizeListener starts a goroutine that listens for SIGWINCH and
// invokes all registered callbacks with the new terminal width.
func startResizeListener() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			width, err := TerminalWidth()
			if err != nil {
				continue
			}
			resizeMu.Lock()
			cbs := make([]func(width int), len(resizeCallbacks))
			copy(cbs, resizeCallbacks)
			resizeMu.Unlock()
			for _, cb := range cbs {
				cb(width)
			}
		}
	}()
}
