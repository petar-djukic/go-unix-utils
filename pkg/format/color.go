// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"io"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// ANSI escape sequences for file type colorization per GNU ls LS_COLORS defaults.
// R2.4: color assignments matching GNU ls defaults.
const (
	colorDirectory  = "\033[34m"   // blue
	colorSymlink    = "\033[36m"   // cyan
	colorExecutable = "\033[32m"   // green
	colorBlockDev   = "\033[33;1m" // yellow bold
	colorCharDev    = "\033[33;1m" // yellow bold
	colorSocket     = "\033[35m"   // magenta
	colorPipe       = "\033[33m"   // yellow
	colorRegular    = "\033[0m"    // reset/default
	colorReset      = "\033[0m"    // reset sequence
)

// colorMu protects the colorOverride variable.
var colorMu sync.RWMutex

// colorOverride stores the process-global color override. A nil value means
// automatic TTY detection; a non-nil value forces color on or off.
// R2.6: process-global override for --color=always and --color=never.
var colorOverride *bool

// FileTypeColor returns the ANSI escape sequence for a file's type based on its
// mode bits. Returns an empty string when color output is disabled.
// R2.1, R2.4: color lookup for all eight GNU ls file types.
func FileTypeColor(mode os.FileMode) string {
	if !colorActive() {
		return ""
	}
	switch {
	case mode.IsDir():
		return colorDirectory
	case mode&os.ModeSymlink != 0:
		return colorSymlink
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return colorCharDev
	case mode&os.ModeDevice != 0:
		return colorBlockDev
	case mode&os.ModeSocket != 0:
		return colorSocket
	case mode&os.ModeNamedPipe != 0:
		return colorPipe
	case mode.IsRegular() && mode.Perm()&0o111 != 0:
		return colorExecutable
	default:
		return colorRegular
	}
}

// Reset returns the ANSI reset escape sequence that terminates color output.
// Returns an empty string when color output is disabled.
// R2.2: reset sequence for terminating color output.
func Reset() string {
	if !colorActive() {
		return ""
	}
	return colorReset
}

// ColorEnabled reports whether the given io.Writer is backed by a terminal.
// It attempts a type assertion to *os.File; if that fails it returns false.
// Otherwise it checks whether the file descriptor is a terminal via the
// TIOCGWINSZ ioctl.
// R2.3: automatic TTY detection for color suppression on non-terminal output.
func ColorEnabled(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isTerminal(f.Fd())
}

// SetColorEnabled overrides the automatic TTY detection. When set to true,
// FileTypeColor and Reset return ANSI sequences regardless of TTY status.
// When set to false, they return empty strings regardless.
// R2.6: process-global override supporting --color=always and --color=never.
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = &enabled
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting to
// automatic TTY detection via ColorEnabled.
// R2.7: revert to automatic detection for test teardown and deferred cleanup.
func ResetColorEnabled() {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = nil
}

// colorActive reports whether color output is currently enabled, checking the
// process-global override first and falling back to automatic TTY detection on
// os.Stdout.
func colorActive() bool {
	colorMu.RLock()
	defer colorMu.RUnlock()
	if colorOverride != nil {
		return *colorOverride
	}
	return ColorEnabled(os.Stdout)
}

// isTerminal reports whether the file descriptor refers to a terminal.
// R2.3: detected via TIOCGWINSZ ioctl per prd003-format specification.
func isTerminal(fd uintptr) bool {
	// winsize struct: Row, Col, Xpixel, Ypixel (4 × uint16 = 8 bytes).
	var ws [8]byte
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws[0]))) //nolint:gosec // ioctl with stack-allocated buffer is safe
	return errno == 0
}
