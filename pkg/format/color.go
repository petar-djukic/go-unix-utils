// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd003-format R2.1–R2.7 (ANSI color output).
package format

import (
	"io"
	"os"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// ANSI SGR escape sequences for file-type colorization.
// Color assignments match GNU ls LS_COLORS defaults per prd003-format R2.4.
const (
	ansiReset         = "\033[0m"
	ansiDirectory     = "\033[34m"   // blue
	ansiSymlink       = "\033[36m"   // cyan
	ansiExecutable    = "\033[32m"   // green
	ansiBlockDevice   = "\033[33;1m" // yellow bold
	ansiCharDevice    = "\033[33;1m" // yellow bold
	ansiSocket        = "\033[35m"   // magenta
	ansiNamedPipe     = "\033[33m"   // yellow
)

// colorOverride holds the package-level color override installed by SetColorEnabled.
// nil means auto-detect via terminal detection.
var (
	colorOverride *bool
	colorMu       sync.Mutex
)

// SetColorEnabled installs a package-level override that forces ColorEnabled,
// FileTypeColor, and Reset to return the given value regardless of terminal
// detection. true forces color on; false forces color off.
//
// prd003-format R2.6.
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	v := enabled
	colorOverride = &v
	colorMu.Unlock()
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting to
// automatic TTY detection via ColorEnabled.
//
// prd003-format R2.7.
func ResetColorEnabled() {
	colorMu.Lock()
	colorOverride = nil
	colorMu.Unlock()
}

// ColorEnabled reports whether color output is appropriate for writer w.
//
// When an override is active (set by SetColorEnabled), it returns the override
// value. Otherwise it performs automatic terminal detection: w must be an
// *os.File whose file descriptor passes pkg/sys.IsTerminal.
//
// prd003-format R2.3.
func ColorEnabled(w io.Writer) bool {
	colorMu.Lock()
	override := colorOverride
	colorMu.Unlock()

	if override != nil {
		return *override
	}

	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// FileTypeColor returns the ANSI SGR escape sequence for the file type encoded
// in mode. Returns an empty string for regular files with no special attributes
// or when color is disabled (no override active and os.Stdout is not a TTY).
//
// Color assignments follow GNU ls LS_COLORS defaults (prd003-format R2.4).
//
// prd003-format R2.1.
func FileTypeColor(mode os.FileMode) string {
	if !colorActive() {
		return ""
	}
	return fileTypeSequence(mode)
}

// Reset returns the ANSI SGR reset escape sequence that clears all active color
// attributes. Returns an empty string when color is disabled.
//
// prd003-format R2.2.
func Reset() string {
	if !colorActive() {
		return ""
	}
	return ansiReset
}

// colorActive reports whether ANSI sequences should currently be emitted,
// consulting the global override and, when absent, os.Stdout terminal state.
func colorActive() bool {
	colorMu.Lock()
	override := colorOverride
	colorMu.Unlock()

	if override != nil {
		return *override
	}
	return sys.IsTerminal(os.Stdout.Fd())
}

// fileTypeSequence returns the raw ANSI escape sequence for mode, or an empty
// string for plain regular files. Called only when color is known to be active.
func fileTypeSequence(mode os.FileMode) string {
	switch {
	case mode.IsDir():
		return ansiDirectory
	case mode&os.ModeSymlink != 0:
		return ansiSymlink
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return ansiCharDevice
	case mode&os.ModeDevice != 0:
		return ansiBlockDevice
	case mode&os.ModeSocket != 0:
		return ansiSocket
	case mode&os.ModeNamedPipe != 0:
		return ansiNamedPipe
	case mode.IsRegular() && mode&0o111 != 0:
		return ansiExecutable
	default:
		// R2.1: regular files with no special attributes return empty string.
		return ""
	}
}
