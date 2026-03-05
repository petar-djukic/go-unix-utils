// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"io"
	"os"
	"syscall"
)

// ANSI escape sequences for file-type colorization.
// Assignments match GNU ls LS_COLORS defaults (prd003-format R2.4).
const (
	ansiDir      = "\033[34m"   // directory: blue (34)
	ansiLink     = "\033[36m"   // symlink: cyan (36)
	ansiExec     = "\033[32m"   // executable regular file: green (32)
	ansiBlockDev = "\033[33;1m" // block device: yellow bold (33;1)
	ansiCharDev  = "\033[33;1m" // character device: yellow bold (33;1)
	ansiSocket   = "\033[35m"   // socket: magenta (35)
	ansiPipe     = "\033[33m"   // named pipe (FIFO): yellow (33)
	ansiReset    = "\033[0m"    // reset — also the indicator for regular files (0)
)

// colorState holds the process-global color override.
// nil means automatic TTY detection; non-nil forces the given value.
// Guarded by process-global convention: only SetColorEnabled and
// ResetColorEnabled write this variable (prd003-format R2.6, R2.7).
var colorState *bool //nolint:gochecknoglobals

// isTerminalFn checks whether a file descriptor refers to a terminal.
// It is a variable so tests can replace it without calling os.Pipe.
var isTerminalFn = defaultIsTerminal //nolint:gochecknoglobals

// defaultIsTerminal reports whether fd is a character device (which covers
// all interactive terminal pseudo-devices on Linux and Darwin).
func defaultIsTerminal(fd uintptr) bool {
	var stat syscall.Stat_t
	if err := syscall.Fstat(int(fd), &stat); err != nil {
		return false
	}
	return stat.Mode&syscall.S_IFMT == syscall.S_IFCHR
}

// SetColorEnabled overrides automatic TTY detection for the process.
// When enabled is true, FileTypeColor and Reset return ANSI sequences
// regardless of whether the output is a TTY (supports --color=always).
// When enabled is false, they return empty strings (supports --color=never).
// Implements prd003-format R2.6.
func SetColorEnabled(enabled bool) {
	colorState = &enabled
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting
// to automatic TTY detection via ColorEnabled. Utilities must call this in
// deferred cleanup or test teardown to avoid leaking state.
// Implements prd003-format R2.7.
func ResetColorEnabled() {
	colorState = nil
}

// ColorEnabled reports whether color output is enabled for the given writer.
// If an override has been set by SetColorEnabled, that value is returned.
// Otherwise it returns true only when w is an *os.File whose file descriptor
// refers to a terminal.
// Implements prd003-format R2.3.
func ColorEnabled(w io.Writer) bool {
	if colorState != nil {
		return *colorState
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isTerminalFn(f.Fd())
}

// colorActive reports whether color output is currently active, using
// os.Stdout for automatic detection when no override is set.
func colorActive() bool {
	return ColorEnabled(os.Stdout)
}

// FileTypeColor returns the ANSI escape sequence for the file type described
// by mode. It returns an empty string when color is disabled.
// Color assignments match GNU ls LS_COLORS defaults (prd003-format R2.1, R2.4).
func FileTypeColor(mode os.FileMode) string {
	if !colorActive() {
		return ""
	}
	switch {
	case mode&os.ModeDir != 0:
		return ansiDir
	case mode&os.ModeSymlink != 0:
		return ansiLink
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return ansiCharDev
	case mode&os.ModeDevice != 0:
		return ansiBlockDev
	case mode&os.ModeSocket != 0:
		return ansiSocket
	case mode&os.ModeNamedPipe != 0:
		return ansiPipe
	case mode&0o111 != 0:
		// Regular file with any execute permission bit set.
		return ansiExec
	default:
		// Regular file: return reset sequence (GNU ls code 0).
		return ansiReset
	}
}

// Reset returns the ANSI terminal reset sequence ("\033[0m") when color is
// active, and an empty string otherwise.
// Implements prd003-format R2.2.
func Reset() string {
	if !colorActive() {
		return ""
	}
	return ansiReset
}
