// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R2.1-R2.7: ANSI color output functions.
package format

import (
	"io"
	"os"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// R2.4: GNU ls LS_COLORS default ANSI SGR codes.
const (
	colorDir    = "\033[34m"   // blue
	colorLink   = "\033[36m"   // cyan
	colorExec   = "\033[32m"   // green
	colorBlock  = "\033[33;1m" // yellow bold
	colorChar   = "\033[33;1m" // yellow bold
	colorSocket = "\033[35m"   // magenta
	colorPipe   = "\033[33m"   // yellow
	colorReset  = "\033[0m"
)

var (
	colorMu       sync.RWMutex
	colorOverride *bool // nil = auto, non-nil = forced
)

// FileTypeColor returns the ANSI escape sequence for a file's type matching
// GNU ls --color=auto conventions. Returns an empty string when color output
// is not active (no override set or override is false).
// Implements prd003-format R2.1, R2.4.
func FileTypeColor(mode os.FileMode) string {
	if !colorActive() {
		return ""
	}
	return fileTypeSequence(mode)
}

// fileTypeSequence returns the ANSI SGR sequence for the given file mode.
func fileTypeSequence(mode os.FileMode) string {
	switch {
	case mode&os.ModeDir != 0:
		return colorDir
	case mode&os.ModeSymlink != 0:
		return colorLink
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return colorChar
	case mode&os.ModeDevice != 0:
		return colorBlock
	case mode&os.ModeSocket != 0:
		return colorSocket
	case mode&os.ModeNamedPipe != 0:
		return colorPipe
	case mode.IsRegular() && mode&0o111 != 0:
		return colorExec
	default:
		return ""
	}
}

// Reset returns the ANSI reset escape sequence. Returns an empty string
// when color output is not active.
// Implements prd003-format R2.2.
func Reset() string {
	if !colorActive() {
		return ""
	}
	return colorReset
}

// ColorEnabled returns true when color output should be used for the given
// writer. If an override is set via SetColorEnabled, returns the override
// value. Otherwise, attempts a type assertion to *os.File and calls
// pkg/sys.IsTerminal to detect whether the writer is a terminal.
// Implements prd003-format R2.3.
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

// SetColorEnabled overrides automatic terminal detection, forcing color on
// or off for all subsequent ColorEnabled, FileTypeColor, and Reset calls.
// The override is process-global.
// Implements prd003-format R2.6.
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	colorOverride = &enabled
	colorMu.Unlock()
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting
// to automatic TTY detection via ColorEnabled.
// Implements prd003-format R2.7.
func ResetColorEnabled() {
	colorMu.Lock()
	colorOverride = nil
	colorMu.Unlock()
}

// colorActive returns true when the package-level color override is
// explicitly set to true. Returns false when no override is set or
// when the override is false.
func colorActive() bool {
	colorMu.RLock()
	override := colorOverride
	colorMu.RUnlock()

	if override != nil {
		return *override
	}
	return false
}
