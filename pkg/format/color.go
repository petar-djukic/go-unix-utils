// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd003-format R2.1–R2.7 (FileTypeColor, Reset, ColorEnabled,
// SetColorEnabled, ResetColorEnabled).
package format

import (
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// ANSI SGR escape sequences matching GNU ls LS_COLORS defaults. R2.4.
const (
	colorDirectory = "\033[34m"  // blue
	colorSymlink   = "\033[36m"  // cyan
	colorExec      = "\033[32m"  // green
	colorBlockDev  = "\033[33;1m" // yellow bold
	colorCharDev   = "\033[33;1m" // yellow bold
	colorSocket    = "\033[35m"  // magenta
	colorPipe      = "\033[33m"  // yellow
	colorReset     = "\033[0m"
)

// colorOverride holds the forced color state set by SetColorEnabled. R2.6, R2.7.
var (
	colorOverrideActive bool
	colorOverrideValue  bool
)

// FileTypeColor returns the ANSI escape sequence for the given file type based
// on its os.FileMode. Returns an empty string for regular files with no special
// mode bits. R2.1, R2.4.
func FileTypeColor(mode os.FileMode) string {
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
		// Executable bit set on a regular file.
		return colorExec
	default:
		return ""
	}
}

// Reset returns the ANSI reset escape sequence that terminates colored
// output. R2.2.
func Reset() string {
	return colorReset
}

// ColorEnabled returns true if color output should be used for the given
// writer. When an override has been set via SetColorEnabled, the override
// value is returned. Otherwise, it returns true only when w is backed by
// a terminal file descriptor. R2.3.
func ColorEnabled(w io.Writer) bool {
	if colorOverrideActive {
		return colorOverrideValue
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// SetColorEnabled forces color output on or off regardless of terminal
// detection. Pass true for --color=always behavior, false for
// --color=never. R2.6.
func SetColorEnabled(enabled bool) {
	colorOverrideActive = true
	colorOverrideValue = enabled
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting
// ColorEnabled to automatic terminal detection. R2.7.
func ResetColorEnabled() {
	colorOverrideActive = false
	colorOverrideValue = false
}
