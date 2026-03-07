// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"io"
	"os"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// ANSI escape prefix and reset sequence.
const (
	ansiEsc      = "\033["
	ansiReset    = "\033[0m"
	ansiSuffix   = "m"
)

// R2.4: GNU ls LS_COLORS default color codes.
const (
	colorDirectory = "34"   // blue
	colorSymlink   = "36"   // cyan
	colorExec      = "32"   // green
	colorBlockDev  = "33;1" // yellow bold
	colorCharDev   = "33;1" // yellow bold
	colorSocket    = "35"   // magenta
	colorPipe      = "33"   // yellow
	colorRegular   = "0"    // reset/default
)

var (
	colorMu       sync.RWMutex
	colorOverride *bool // nil = auto-detect, non-nil = forced value
)

// ColorEnabled returns true if color output should be used for the given writer.
// It checks if w is backed by an *os.File whose fd is a terminal via
// pkg/sys.IsTerminal. If SetColorEnabled has been called, the override value
// is returned instead. (prd003-format R2.3)
func ColorEnabled(w io.Writer) bool {
	colorMu.RLock()
	override := colorOverride
	colorMu.RUnlock()

	if override != nil {
		return *override
	}

	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// SetColorEnabled overrides automatic TTY detection. When enabled is true,
// FileTypeColor and Reset return ANSI sequences regardless of terminal state.
// When false, they return empty strings. (prd003-format R2.6)
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	colorOverride = &enabled
	colorMu.Unlock()
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting to
// automatic TTY detection. (prd003-format R2.7)
func ResetColorEnabled() {
	colorMu.Lock()
	colorOverride = nil
	colorMu.Unlock()
}

// FileTypeColor returns the ANSI escape sequence for the given file mode's type,
// using GNU ls LS_COLORS defaults. Returns an empty string if color is not
// enabled (auto-detection or override). (prd003-format R2.1, R2.4)
func FileTypeColor(mode os.FileMode) string {
	if !colorActive() {
		return ""
	}

	var code string
	switch {
	case mode&os.ModeDir != 0:
		code = colorDirectory
	case mode&os.ModeSymlink != 0:
		code = colorSymlink
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		code = colorCharDev
	case mode&os.ModeDevice != 0:
		code = colorBlockDev
	case mode&os.ModeSocket != 0:
		code = colorSocket
	case mode&os.ModeNamedPipe != 0:
		code = colorPipe
	case mode.Perm()&0o111 != 0:
		code = colorExec
	default:
		code = colorRegular
	}
	return ansiEsc + code + ansiSuffix
}

// Reset returns the ANSI reset sequence. Returns an empty string if color is
// not enabled. (prd003-format R2.2)
func Reset() string {
	if !colorActive() {
		return ""
	}
	return ansiReset
}

// colorActive checks the override state. When no override is set, it defaults
// to false (callers that need writer-specific detection should use ColorEnabled).
func colorActive() bool {
	colorMu.RLock()
	override := colorOverride
	colorMu.RUnlock()

	if override != nil {
		return *override
	}
	return false
}
