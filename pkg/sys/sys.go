// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sys provides portable syscall abstractions for Unix utilities:
// terminal-width queries (R1), extended file metadata (R2), and signal
// handling for SIGPIPE and SIGWINCH (R1, R3).
//
// Implements: prd002-sys
// Architecture: docs/ARCHITECTURE.yaml § pkg/sys
package sys

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// FileInfo holds extended file metadata not available from os.FileInfo.
// Stat and Lstat populate all fields from the platform syscall.Stat_t.
// Implements prd002-sys R2.2.
type FileInfo struct {
	// Mode holds the file mode and type bits (from syscall.Stat_t.Mode).
	Mode os.FileMode
	// Size is the apparent file size in bytes (st_size).
	Size int64
	// Nlink is the hard-link count (st_nlink).
	Nlink uint64
	// Uid is the owner user ID (st_uid).
	// Field name follows syscall.Stat_t convention (not UID) per prd002-sys R2.2.
	Uid uint32 //nolint:revive
	// Gid is the owner group ID (st_gid).
	Gid uint32
	// Rdev is the device ID for special files (st_rdev).
	Rdev uint64
	// Dev is the device ID of the containing filesystem (st_dev).
	Dev uint64
	// Ino is the inode number (st_ino).
	Ino uint64
	// Blocks is the number of 512-byte blocks allocated (st_blocks).
	Blocks int64
	// Blksize is the preferred I/O block size (st_blksize).
	Blksize int64
	// ModTime is the modification time (st_mtime / st_mtimespec).
	ModTime time.Time
	// Info is the underlying os.FileInfo for os package compatibility.
	Info os.FileInfo
}

// TerminalWidth returns the current terminal column count by calling
// ioctl(TIOCGWINSZ) on stdout. Returns an error when stdout is not a
// terminal or when the ioctl fails.
// Implements prd002-sys R1.1, R1.2.
func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, fmt.Errorf("TerminalWidth: ioctl TIOCGWINSZ: %w", err)
	}
	return int(ws.Col), nil
}

// IsTerminal reports whether the file descriptor refers to a terminal.
// Returns false for pipes, regular files, and non-terminal descriptors.
// The implementation calls IoctlGetWinsize; a successful ioctl indicates a TTY.
// Implements prd002-sys R1.3.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}

// InstallSIGPIPEHandler installs a SIGPIPE handler that exits 0 when a
// downstream consumer closes stdout or stderr. Uses signal.Notify (not
// signal.Ignore) so that deferred cleanup functions do not run on SIGPIPE,
// matching GNU behavior. Safe to call multiple times; each call starts one
// goroutine.
// Implements prd002-sys R1.5, R1.6.
func InstallSIGPIPEHandler() {
	// R1.5: buffered channel of size 1; goroutine exits the process on receive.
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}

// resizeMu guards resizeCallbacks.
var resizeMu sync.Mutex //nolint:gochecknoglobals

// resizeCallbacks holds all callbacks registered via OnTerminalResize.
var resizeCallbacks []func(width int) //nolint:gochecknoglobals

// resizeOnce ensures the SIGWINCH goroutine is started exactly once.
var resizeOnce sync.Once //nolint:gochecknoglobals

// OnTerminalResize registers callback to be called with the new terminal
// width whenever the terminal is resized (SIGWINCH). Supports multiple
// calls; each registered callback is invoked in registration order.
// Implements prd002-sys R3.1, R3.2.
func OnTerminalResize(callback func(width int)) {
	resizeMu.Lock()
	resizeCallbacks = append(resizeCallbacks, callback)
	resizeMu.Unlock()

	resizeOnce.Do(func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGWINCH)
		go func() {
			for range c {
				width, err := TerminalWidth()
				if err != nil {
					// R3.1: do not invoke callbacks when TerminalWidth fails.
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
	})
}

// Stat returns extended file metadata for path, following symbolic links.
// Equivalent to os.Stat but also populates platform-specific fields
// (Nlink, Uid, Gid, Rdev, Dev, Ino, Blocks, Blksize, ModTime).
// Implements prd002-sys R2.1.
func Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("sys.Stat: unexpected Sys() type for %s", path)
	}
	fi := &FileInfo{
		Mode: info.Mode(),
		Size: info.Size(),
		Info: info,
	}
	fillFromStat(fi, st)
	return fi, nil
}

// Lstat returns extended file metadata for path without following symbolic
// links. Equivalent to os.Lstat but also populates platform-specific fields.
// When path is a symlink, the returned FileInfo describes the symlink itself.
// Implements prd002-sys R2.1, R2.3.
func Lstat(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("sys.Lstat: unexpected Sys() type for %s", path)
	}
	fi := &FileInfo{
		Mode: info.Mode(),
		Size: info.Size(),
		Info: info,
	}
	fillFromStat(fi, st)
	return fi, nil
}
