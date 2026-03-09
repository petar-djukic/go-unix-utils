// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"io"
	"os"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// ANSI escape sequences matching GNU ls LS_COLORS defaults. R2.4.
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

// colorOverrideMu protects the color override state.
var colorOverrideMu sync.RWMutex

// colorOverride holds the manual override value. nil means auto-detect.
var colorOverride *bool

// FileTypeColor returns the ANSI escape sequence for the given file mode's type.
// R2.1, R2.4: maps directory, symlink, executable, block/char device, socket, pipe
// to GNU ls default colors. Returns empty string for regular files or when color
// is disabled.
func FileTypeColor(mode os.FileMode) string {
	if !isColorActive() {
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
	case mode&0o111 != 0 && mode.IsRegular():
		return colorExec
	default:
		return ""
	}
}

// Reset returns the ANSI reset sequence, or empty string when color is disabled.
// R2.2.
func Reset() string {
	if !isColorActive() {
		return ""
	}
	return colorReset
}

// ColorEnabled returns true when w is backed by a terminal file descriptor.
// R2.3: type-asserts to *os.File and calls pkg/sys.IsTerminal.
func ColorEnabled(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// SetColorEnabled overrides automatic TTY detection. R2.6: when true, ANSI
// sequences are always returned; when false, they are always suppressed.
func SetColorEnabled(enabled bool) {
	colorOverrideMu.Lock()
	colorOverride = &enabled
	colorOverrideMu.Unlock()
}

// ResetColorEnabled clears the override set by SetColorEnabled, reverting to
// automatic TTY detection. R2.7.
func ResetColorEnabled() {
	colorOverrideMu.Lock()
	colorOverride = nil
	colorOverrideMu.Unlock()
}

// isColorActive returns whether color output is currently active, considering
// any override set by SetColorEnabled.
func isColorActive() bool {
	colorOverrideMu.RLock()
	ov := colorOverride
	colorOverrideMu.RUnlock()
	if ov != nil {
		return *ov
	}
	return ColorEnabled(os.Stdout)
}
