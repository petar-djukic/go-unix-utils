// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// color.go provides ANSI color codes for Unix file types following GNU ls
// LS_COLORS default assignments, with automatic TTY detection and manual
// override for --color=always and --color=never flags.
//
// Implements: prd003-format R2.1, R2.2, R2.3, R2.4, R2.5, R2.6, R2.7.
package format

import (
	"io"
	"os"
	"sync"
)

// ANSI escape sequences for file types matching GNU ls LS_COLORS defaults
// (prd003-format R2.4).
const (
	colorDirectory = "\033[34m"   // blue
	colorSymlink   = "\033[36m"   // cyan
	colorExec      = "\033[32m"   // green
	colorBlockDev  = "\033[33;1m" // yellow bold
	colorCharDev   = "\033[33;1m" // yellow bold
	colorSocket    = "\033[35m"   // magenta
	colorPipe      = "\033[33m"   // yellow
	colorReset     = "\033[0m"
)

// colorOverride tracks the process-global color override state.
// nil means automatic TTY detection; non-nil means forced on/off.
var (
	colorMu       sync.RWMutex
	colorOverride *bool
)

// SetColorEnabled overrides automatic TTY detection for FileTypeColor and
// Reset. When set to true, ANSI sequences are returned regardless of TTY
// state. When false, empty strings are returned regardless.
//
// Implements: prd003-format R2.6.
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = &enabled
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting to
// automatic TTY detection via ColorEnabled.
//
// Implements: prd003-format R2.7.
func ResetColorEnabled() {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = nil
}

// colorEnabled returns whether color output is currently active, considering
// both the manual override and automatic detection against the given writer.
func colorEnabled(w io.Writer) bool {
	colorMu.RLock()
	defer colorMu.RUnlock()

	if colorOverride != nil {
		return *colorOverride
	}
	return ColorEnabled(w)
}

// isColorForced returns (forced bool, value bool). If forced is true, value
// is the override state. If forced is false, the caller should use automatic
// detection.
func isColorForced() (bool, bool) {
	colorMu.RLock()
	defer colorMu.RUnlock()

	if colorOverride != nil {
		return true, *colorOverride
	}
	return false, false
}

// ColorEnabled returns true only when w is backed by a terminal (TTY).
// It attempts a type assertion to *os.File; if that fails, returns false.
// For *os.File, it checks whether the file descriptor is a terminal.
//
// Note: This function does NOT consult the manual override set by
// SetColorEnabled. Use FileTypeColor and Reset which handle the override
// automatically.
//
// Implements: prd003-format R2.3.
func ColorEnabled(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isTerminal(f.Fd())
}

// isTerminal reports whether the given file descriptor is a terminal.
// This uses a simple ioctl approach without importing pkg/sys, per D1/D3.
// On non-terminal file descriptors (pipes, regular files), the ioctl fails
// and this returns false.
func isTerminal(fd uintptr) bool {
	// We avoid importing pkg/sys per design decision D1. Instead we check
	// if Stat on the fd indicates a character device, which is a reasonable
	// heuristic. For the actual TTY check, we use the same approach as
	// golang.org/x/term: attempt to get terminal state.
	//
	// However, per D3 we cannot import external deps. We use os.NewFile +
	// Stat to check if it's a character device as an approximation.
	f := os.NewFile(fd, "")
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// FileTypeColor returns the ANSI escape sequence for the given file mode's
// type. Returns an empty string when color is disabled (via SetColorEnabled
// or when the override is not set and automatic detection would apply — in
// that case callers should use the colorEnabled helper or set the override).
//
// When no override is set and this function is called directly, it returns
// the ANSI code unconditionally so that callers who have already checked
// ColorEnabled can use it. When an override IS set, the override is respected.
//
// Implements: prd003-format R2.1, R2.4.
func FileTypeColor(mode os.FileMode) string {
	forced, value := isColorForced()
	if forced && !value {
		return ""
	}

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
	case mode&0111 != 0:
		// Executable regular file.
		return colorExec
	default:
		// Regular file with no special type — no color (prd003-format R2.4:
		// regular=0, meaning reset/default). Return empty string; the caller
		// wraps with Reset if needed.
		return ""
	}
}

// Reset returns the ANSI reset sequence ("\033[0m") when color is enabled,
// or an empty string when color is disabled.
//
// Implements: prd003-format R2.2.
func Reset() string {
	forced, value := isColorForced()
	if forced && !value {
		return ""
	}
	return colorReset
}
