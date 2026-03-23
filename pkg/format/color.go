// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R2.1–R2.7: ANSI color output for file types,
// terminal detection, and color override control.
package format

import (
	"io"
	"os"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// ANSI escape sequences for file type colorization.
// R2.4: matches GNU ls LS_COLORS defaults.
const (
	ansiReset     = "\033[0m"
	ansiDirectory = "\033[34m"
	ansiSymlink   = "\033[36m"
	ansiExec      = "\033[32m"
	ansiBlockDev  = "\033[33;1m"
	ansiCharDev   = "\033[33;1m"
	ansiSocket    = "\033[35m"
	ansiPipe      = "\033[33m"
)

// colorOverride holds the process-global color override state.
// R2.6–R2.7: SetColorEnabled/ResetColorEnabled control this.
var (
	colorMu       sync.RWMutex
	colorOverride *bool // nil = auto-detect, non-nil = forced value
)

// FileTypeColor returns the ANSI escape sequence for a file's type.
//
// R2.1: returns the escape sequence based on file mode type bits.
// R2.4: uses GNU ls default color assignments.
// R2.3/R2.6: returns empty string when color is disabled.
func FileTypeColor(mode os.FileMode) string {
	if !isColorActive() {
		return ""
	}
	return fileTypeEscape(mode)
}

// fileTypeEscape returns the raw ANSI escape for the given file mode.
func fileTypeEscape(mode os.FileMode) string {
	switch {
	case mode&os.ModeDir != 0:
		return ansiDirectory
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
		return ansiReset
	}
}

// Reset returns the ANSI reset escape sequence.
//
// R2.2: returns "\033[0m" when color is active, empty string otherwise.
func Reset() string {
	if !isColorActive() {
		return ""
	}
	return ansiReset
}

// ColorEnabled reports whether color output should be used for w.
//
// R2.3: returns true only when w is backed by a terminal file descriptor.
// Uses sys.IsTerminal for the TTY check.
func ColorEnabled(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// SetColorEnabled overrides automatic TTY detection.
//
// R2.6: when enabled is true, FileTypeColor and Reset return ANSI sequences
// regardless of TTY status. When false, they return empty strings.
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = &enabled
}

// ResetColorEnabled clears any override set by SetColorEnabled,
// reverting to automatic TTY detection via ColorEnabled.
//
// R2.7: restores auto-detection behavior.
func ResetColorEnabled() {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = nil
}

// isColorActive returns the effective color state, checking the override
// first, then falling back to TTY detection on stdout.
func isColorActive() bool {
	colorMu.RLock()
	override := colorOverride
	colorMu.RUnlock()

	if override != nil {
		return *override
	}
	return ColorEnabled(os.Stdout)
}
