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
	colorRegular   = "\033[0m"   // reset/default
	ansiReset      = "\033[0m"
)

// colorMu protects the override state.
var colorMu sync.RWMutex

// colorOverride stores the forced color state when set.
var colorOverride *bool

// FileTypeColor returns the ANSI escape sequence for the given file mode's
// type, matching GNU ls LS_COLORS defaults. R2.1, R2.4.
//
// When color is disabled (via override or non-TTY), returns an empty string. R2.3.
func FileTypeColor(mode os.FileMode) string {
	if !colorEnabled() {
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
	case mode.Perm()&0o111 != 0:
		return colorExec
	default:
		return colorRegular
	}
}

// Reset returns the ANSI reset escape sequence. R2.2.
//
// When color is disabled, returns an empty string.
func Reset() string {
	if !colorEnabled() {
		return ""
	}
	return ansiReset
}

// ColorEnabled returns true when the writer is a terminal and no override
// forces color off. R2.3.
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

// SetColorEnabled overrides automatic TTY detection for FileTypeColor and
// Reset. R2.6.
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	colorOverride = &enabled
	colorMu.Unlock()
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting
// to automatic TTY detection. R2.7.
func ResetColorEnabled() {
	colorMu.Lock()
	colorOverride = nil
	colorMu.Unlock()
}

// colorEnabled returns the effective color state using the override or
// defaulting to checking os.Stdout as a TTY.
func colorEnabled() bool {
	colorMu.RLock()
	override := colorOverride
	colorMu.RUnlock()
	if override != nil {
		return *override
	}
	return sys.IsTerminal(os.Stdout.Fd())
}
