// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R2: ANSI color helpers for file types.

package format

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

// ANSI escape sequences for file types, matching GNU ls LS_COLORS defaults
// (prd003-format R2.4). The regular file code is 0 (reset/default).
const (
	ansiReset    = "\033[0m"
	ansiDir      = "\033[34m"   // directory: blue
	ansiSymlink  = "\033[36m"   // symlink: cyan
	ansiExec     = "\033[32m"   // executable: green
	ansiBlockDev = "\033[33;1m" // block device: yellow bold
	ansiCharDev  = "\033[33;1m" // character device: yellow bold
	ansiSocket   = "\033[35m"   // socket: magenta
	ansiPipe     = "\033[33m"   // pipe/FIFO: yellow
	ansiRegular  = ansiReset    // regular file: reset/default (code 0 per R2.4)
)

// colorOverride controls the global color-enabled state.
// nil = auto-detect via TTY check; non-nil = forced value.
// (prd003-format R2.6, R2.7)
var colorOverride *bool

// SetColorEnabled overrides automatic TTY detection for ANSI color output.
// When enabled is true, FileTypeColor and Reset return ANSI sequences
// regardless of whether stdout is a TTY. When enabled is false, they return
// empty strings regardless. Supports --color=always and --color=never.
// The override is process-global. (prd003-format R2.6)
func SetColorEnabled(enabled bool) {
	v := enabled
	colorOverride = &v
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting to
// automatic TTY detection via ColorEnabled. Utilities must call this in
// deferred cleanup or test teardown to avoid leaking state across test cases.
// (prd003-format R2.7)
func ResetColorEnabled() {
	colorOverride = nil
}

// ColorEnabled reports whether w supports ANSI color output by checking
// whether it is a TTY. Returns false for non-*os.File writers (pipes,
// buffers, etc.) and for file descriptors that are not terminals.
// This function does not consult the SetColorEnabled override; use it to
// query the raw TTY state of a specific writer. (prd003-format R2.3)
func ColorEnabled(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isTerminal(f.Fd())
}

// colorActive returns the effective color-enabled state: the forced override
// when set, or the TTY state of os.Stdout otherwise.
func colorActive() bool {
	if colorOverride != nil {
		return *colorOverride
	}
	return ColorEnabled(os.Stdout)
}

// FileTypeColor returns the ANSI escape sequence for the file mode's type,
// or an empty string when color output is disabled. File type is determined
// from the mode bits; permission bits determine executable status.
// (prd003-format R2.1, R2.3, R2.4)
func FileTypeColor(mode os.FileMode) string {
	if !colorActive() {
		return ""
	}
	return fileTypeANSI(mode)
}

// Reset returns the ANSI reset sequence ("\033[0m"), or an empty string when
// color output is disabled. (prd003-format R2.2)
func Reset() string {
	if !colorActive() {
		return ""
	}
	return ansiReset
}

// fileTypeANSI returns the raw ANSI escape sequence for the given mode
// without consulting the color-enabled state.
func fileTypeANSI(mode os.FileMode) string {
	switch {
	case mode&os.ModeDir != 0:
		return ansiDir
	case mode&os.ModeSymlink != 0:
		return ansiSymlink
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return ansiCharDev
	case mode&os.ModeDevice != 0:
		return ansiBlockDev
	case mode&os.ModeSocket != 0:
		return ansiSocket
	case mode&os.ModeNamedPipe != 0:
		return ansiPipe
	case mode&0o111 != 0:
		return ansiExec
	default:
		return ansiRegular
	}
}

// isTerminal reports whether fd is a terminal by attempting a TIOCGWINSZ
// ioctl. A successful ioctl indicates the descriptor is a terminal; any
// error (ENOTTY, EBADF, etc.) indicates it is not.
// Uses only the standard library syscall package. (prd003-format R2.3)
func isTerminal(fd uintptr) bool {
	// struct winsize: rows, cols, xpixel, ypixel (each uint16)
	var ws [4]uint16
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws[0])),
	)
	return errno == 0
}
