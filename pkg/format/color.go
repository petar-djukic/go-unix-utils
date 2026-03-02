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

// ANSI escape codes for file types matching GNU ls LS_COLORS defaults.
// prd003-format R2.4.
const (
	colorDir     = "\033[34m"   // blue
	colorSymlink = "\033[36m"   // cyan
	colorExec    = "\033[32m"   // green
	colorBlock   = "\033[33;1m" // yellow bold
	colorChar    = "\033[33;1m" // yellow bold
	colorSocket  = "\033[35m"   // magenta
	colorPipe    = "\033[33m"   // yellow
	ansiReset    = "\033[0m"
)

var (
	colorMu       sync.RWMutex
	colorOverride *bool // nil = auto, non-nil = forced
)

// SetColorEnabled overrides automatic TTY detection. When set to true,
// FileTypeColor and Reset return ANSI sequences regardless of whether stdout
// is a TTY. When set to false, they return empty strings regardless. The
// override is process-global.
//
// prd003-format R2.6.
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = &enabled
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting to
// automatic TTY detection via ColorEnabled. Utilities should call this in
// deferred cleanup or test teardown to avoid leaking the override.
//
// prd003-format R2.7.
func ResetColorEnabled() {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = nil
}

// ColorEnabled returns true when w is backed by a TTY file descriptor. It
// attempts a type assertion to *os.File and checks the descriptor with
// TIOCGWINSZ. Non-*os.File writers and pipe descriptors return false.
//
// prd003-format R2.3.
func ColorEnabled(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isTerminal(f.Fd())
}

// isColorActive checks the process-global color state. When an override is
// set, it returns the override value. In auto mode, it checks os.Stdout.
func isColorActive() bool {
	colorMu.RLock()
	defer colorMu.RUnlock()
	if colorOverride != nil {
		return *colorOverride
	}
	return isTerminal(os.Stdout.Fd())
}

// FileTypeColor returns the ANSI escape sequence for a file's type. Returns
// an empty string for regular files (default terminal color) and when color
// output is disabled.
//
// prd003-format R2.1, R2.4.
func FileTypeColor(mode os.FileMode) string {
	if !isColorActive() {
		return ""
	}
	switch {
	case mode&os.ModeDir != 0:
		return colorDir
	case mode&os.ModeSymlink != 0:
		return colorSymlink
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return colorChar
	case mode&os.ModeDevice != 0:
		return colorBlock
	case mode&os.ModeSocket != 0:
		return colorSocket
	case mode&os.ModeNamedPipe != 0:
		return colorPipe
	case mode.Perm()&0111 != 0:
		return colorExec
	default:
		return ""
	}
}

// Reset returns the ANSI reset sequence ("\033[0m") when color output is
// enabled, or an empty string when disabled.
//
// prd003-format R2.2.
func Reset() string {
	if !isColorActive() {
		return ""
	}
	return ansiReset
}

// isTerminal reports whether fd refers to a terminal. Uses the TIOCGWINSZ
// ioctl which succeeds only on terminal file descriptors.
//
// Note: when pkg/sys is available on this branch, this should delegate to
// pkg/sys.IsTerminal per prd003-format R2.3.
func isTerminal(fd uintptr) bool {
	var ws [4]uint16
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws[0])))
	return errno == 0
}
