// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

var colorOverride *bool

// FileTypeColor returns the ANSI escape sequence for a file's type.
func FileTypeColor(mode os.FileMode) string {
	if colorOverride != nil && !*colorOverride {
		return ""
	}

	switch mode.Type() {
	case os.ModeDir:
		return "\033[01;34m"
	case os.ModeSymlink:
		return "\033[01;36m"
	case os.ModeDevice:
		return "\033[01;33m"
	case os.ModeDevice | os.ModeCharDevice:
		return "\033[01;33m"
	case os.ModeSocket:
		return "\033[01;35m"
	case os.ModeNamedPipe:
		return "\033[40;33m"
	default:
		if mode&0111 != 0 {
			return "\033[01;32m"
		}
		return ""
	}
}

// Reset returns the ANSI reset sequence.
func Reset() string {
	if colorOverride != nil && !*colorOverride {
		return ""
	}
	return "\033[0m"
}

// ColorEnabled reports whether color output is enabled for the given writer.
func ColorEnabled(w io.Writer) bool {
	if colorOverride != nil {
		return *colorOverride
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// SetColorEnabled forces color output on or off regardless of terminal detection.
func SetColorEnabled(enabled bool) {
	colorOverride = &enabled
}

// ResetColorEnabled clears any color override, restoring terminal-based detection.
func ResetColorEnabled() {
	colorOverride = nil
}
