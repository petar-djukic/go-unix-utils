// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R2.1-R2.4, R2.6, R2.7 (ANSI color output).

package format

import (
	"io"
	"os"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// colorOverride holds the process-global color override state.
// When set is true, the forced value overrides automatic TTY detection.
// When set is false, ColorEnabled falls back to TTY detection.
var colorOverride struct {
	mu     sync.RWMutex
	set    bool
	forced bool
}

// ANSI escape sequences for file types, matching GNU ls LS_COLORS defaults (R2.4).
const (
	colorDirectory = "\033[34m"     // blue
	colorSymlink   = "\033[36m"     // cyan
	colorExec      = "\033[32m"     // green
	colorBlockDev  = "\033[33;1m"   // yellow bold
	colorCharDev   = "\033[33;1m"   // yellow bold
	colorSocket    = "\033[35m"     // magenta
	colorPipe      = "\033[33m"     // yellow
	colorSetuid    = "\033[37;41m"  // white on red
	colorSetgid    = "\033[30;43m"  // black on yellow
	colorSticky    = "\033[37;44m"  // white on blue
	colorReset     = "\033[0m"      // reset
)

// ColorEnabled returns true when w is backed by a terminal file descriptor,
// using pkg/sys.IsTerminal for detection. If SetColorEnabled has been called,
// the forced value is returned instead of performing TTY detection. (R2.1, R2.3)
func ColorEnabled(w io.Writer) bool {
	colorOverride.mu.RLock()
	if colorOverride.set {
		v := colorOverride.forced
		colorOverride.mu.RUnlock()
		return v
	}
	colorOverride.mu.RUnlock()

	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// SetColorEnabled overrides automatic TTY detection with a forced value.
// When enabled is true, FileTypeColor and Reset return ANSI sequences
// regardless of the output writer. When false, they return empty strings.
// The override is process-global. (R2.6)
func SetColorEnabled(enabled bool) {
	colorOverride.mu.Lock()
	colorOverride.set = true
	colorOverride.forced = enabled
	colorOverride.mu.Unlock()
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting
// to automatic TTY detection via ColorEnabled. (R2.7)
func ResetColorEnabled() {
	colorOverride.mu.Lock()
	colorOverride.set = false
	colorOverride.forced = false
	colorOverride.mu.Unlock()
}

// isColorForced returns the forced color state if an override is set.
// Returns (forced value, true) when an override is active, or (false, false)
// when automatic detection is in effect.
func isColorForced() (bool, bool) {
	colorOverride.mu.RLock()
	defer colorOverride.mu.RUnlock()
	if colorOverride.set {
		return colorOverride.forced, true
	}
	return false, false
}

// FileTypeColor returns the ANSI escape sequence for the given file mode's
// type, matching GNU ls LS_COLORS defaults (R2.1, R2.4). Returns an empty
// string for regular files. When SetColorEnabled(false) has been called,
// returns an empty string for all file types. (R2.3)
func FileTypeColor(mode os.FileMode) string {
	if forced, ok := isColorForced(); ok && !forced {
		return ""
	}

	// R2.4: check special permission bits before type bits, matching GNU ls
	// priority order (setuid > setgid > sticky > type-based color).
	if mode&os.ModeSetuid != 0 {
		return colorSetuid
	}
	if mode&os.ModeSetgid != 0 {
		return colorSetgid
	}
	if mode&os.ModeSticky != 0 {
		return colorSticky
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
	case mode.IsRegular() && mode&0o111 != 0:
		return colorExec
	default:
		// R2.4: regular files use reset/default — return empty string.
		return ""
	}
}

// Reset returns the ANSI reset escape sequence ("\033[0m") that terminates
// color attributes (R2.2). When SetColorEnabled(false) has been called,
// returns an empty string. (R2.3)
func Reset() string {
	if forced, ok := isColorForced(); ok && !forced {
		return ""
	}
	return colorReset
}
