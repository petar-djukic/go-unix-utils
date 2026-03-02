// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"io"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// ANSI color escape sequences matching GNU ls LS_COLORS defaults
// (prd003-format R2.4).
const (
	colorDirectory = "\033[34m"    // blue
	colorSymlink   = "\033[36m"    // cyan
	colorExec      = "\033[32m"    // green
	colorBlockDev  = "\033[33;1m"  // yellow bold
	colorCharDev   = "\033[33;1m"  // yellow bold
	colorSocket    = "\033[35m"    // magenta
	colorPipe      = "\033[33m"    // yellow
	colorReset     = "\033[0m"
)

// colorOverride tracks whether color output is forced on/off.
var (
	colorMu       sync.Mutex
	colorOverride *bool // nil = auto, non-nil = forced value
)

// SetColorEnabled overrides automatic TTY detection. When set to true,
// FileTypeColor and Reset return ANSI sequences regardless of TTY status.
// When set to false, they return empty strings regardless. The override is
// process-global.
//
// Implements: prd003-format R2.6
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = &enabled
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting
// to automatic TTY detection via ColorEnabled.
//
// Implements: prd003-format R2.7
func ResetColorEnabled() {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = nil
}

// isColorEnabled returns the effective color state: forced value if set,
// otherwise automatic TTY detection on w.
func isColorEnabled(w io.Writer) bool {
	colorMu.Lock()
	override := colorOverride
	colorMu.Unlock()

	if override != nil {
		return *override
	}
	return ColorEnabled(w)
}

// ColorEnabled returns true only when w is a TTY. It attempts a type
// assertion to *os.File; if that fails it returns false. Otherwise it
// checks whether the file descriptor is a terminal.
//
// Implements: prd003-format R2.3
func ColorEnabled(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isTerminal(f.Fd())
}

// isTerminal checks whether fd is a terminal using the TIOCGWINSZ ioctl.
func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}

// FileTypeColor returns the ANSI escape sequence for the given file mode's
// type. Returns an empty string for regular files or when color is disabled.
// The caller should pass os.Stdout (or the active output writer) to enable
// automatic TTY detection; alternatively, use SetColorEnabled to force
// color on or off.
//
// Implements: prd003-format R2.1, R2.4
func FileTypeColor(mode os.FileMode) string {
	if !isColorEnabled(os.Stdout) {
		return ""
	}
	return fileTypeColorCode(mode)
}

// fileTypeColorCode returns the raw ANSI color code for the file type
// without checking whether color is enabled.
func fileTypeColorCode(mode os.FileMode) string {
	switch {
	case mode&os.ModeDir != 0:
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
	case mode&0o111 != 0:
		return colorExec
	default:
		return ""
	}
}

// Reset returns the ANSI reset sequence when color is enabled, or an empty
// string when color is disabled.
//
// Implements: prd003-format R2.2
func Reset() string {
	if !isColorEnabled(os.Stdout) {
		return ""
	}
	return colorReset
}
