// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"io"
	"os"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// ANSI escape sequences for file types matching GNU ls LS_COLORS defaults (R2.4).
const (
	colorDirectory = "\033[34m"  // blue
	colorSymlink   = "\033[36m"  // cyan
	colorExec      = "\033[32m"  // green
	colorBlockDev  = "\033[33;1m" // yellow bold
	colorCharDev   = "\033[33;1m" // yellow bold
	colorSocket    = "\033[35m"  // magenta
	colorPipe      = "\033[33m"  // yellow
	colorRegular   = "\033[0m"   // reset/default
	ansiReset      = "\033[0m"
)

var (
	colorOverrideMu sync.Mutex
	colorOverride   *bool // nil = auto-detect, non-nil = forced value
)

// FileTypeColor returns the ANSI escape sequence for the given file mode's type.
// R2.1, R2.4: uses GNU ls LS_COLORS default assignments.
// When color is disabled (via ColorEnabled check or SetColorEnabled(false)),
// callers should check ColorEnabled before using the returned value.
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
	case mode.IsRegular() && mode&0o111 != 0:
		return colorExec
	default:
		return colorRegular
	}
}

// Reset returns the ANSI reset escape sequence.
// R2.2: returns "\033[0m".
func Reset() string {
	return ansiReset
}

// ColorEnabled returns true when w is a terminal file descriptor.
// R2.3: type-asserts w to *os.File; if it fails, returns false.
// Uses pkg/sys.IsTerminal for terminal detection.
// D2: respects override set by SetColorEnabled.
func ColorEnabled(w io.Writer) bool {
	colorOverrideMu.Lock()
	override := colorOverride
	colorOverrideMu.Unlock()

	if override != nil {
		return *override
	}

	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// SetColorEnabled overrides automatic TTY detection for ColorEnabled.
// R2.6: when set to true, color functions return ANSI sequences regardless of TTY.
// When set to false, they return empty strings regardless.
func SetColorEnabled(enabled bool) {
	colorOverrideMu.Lock()
	defer colorOverrideMu.Unlock()
	colorOverride = &enabled
}

// ResetColorEnabled clears the override set by SetColorEnabled, reverting
// to automatic TTY detection.
// R2.7: utilities call this in deferred cleanup or test teardown.
func ResetColorEnabled() {
	colorOverrideMu.Lock()
	defer colorOverrideMu.Unlock()
	colorOverride = nil
}
