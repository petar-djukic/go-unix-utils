// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// color.go implements prd003 R2.1–R2.7: ANSI color output functions for
// file-type colorization with automatic TTY detection and manual override.

package format

import (
	"io"
	"os"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// ANSI escape sequences for file types per GNU ls LS_COLORS defaults.
// R2.4 (prd003): color assignments match GNU ls defaults.
const (
	colorDir      = "\033[34m"    // blue
	colorSymlink  = "\033[36m"    // cyan
	colorExec     = "\033[32m"    // green
	colorBlockDev = "\033[33;1m"  // yellow bold
	colorCharDev  = "\033[33;1m"  // yellow bold
	colorSocket   = "\033[35m"    // magenta
	colorPipe     = "\033[33m"    // yellow
	colorSetuid   = "\033[37;41m" // white on red
	colorSetgid   = "\033[30;43m" // black on yellow
	colorSticky   = "\033[37;44m" // white on blue
	colorReset    = "\033[0m"
)

// colorState represents the three-state color override.
type colorState int

const (
	colorAuto colorState = iota
	colorForceOn
	colorForceOff
)

var (
	colorMu       sync.Mutex
	colorOverride = colorAuto
)

// FileTypeColor returns the ANSI escape sequence for the given file mode.
//
// R2.1 (prd003): maps file type (directory, symlink, executable, etc.) to color codes.
// R2.3 (prd003): returns empty string when color is not active.
func FileTypeColor(mode os.FileMode) string {
	if !isColorActive() {
		return ""
	}
	return fileTypeCode(mode)
}

// fileTypeCode returns the raw ANSI escape for the given mode.
// R2.4 (prd003): uses GNU ls LS_COLORS default assignments.
func fileTypeCode(mode os.FileMode) string {
	switch {
	case mode&os.ModeSymlink != 0:
		return colorSymlink
	case mode&os.ModeDir != 0 && mode&os.ModeSticky != 0:
		return colorSticky
	case mode&os.ModeDir != 0:
		return colorDir
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		return colorCharDev
	case mode&os.ModeDevice != 0:
		return colorBlockDev
	case mode&os.ModeSocket != 0:
		return colorSocket
	case mode&os.ModeNamedPipe != 0:
		return colorPipe
	case mode&os.ModeSetuid != 0:
		return colorSetuid
	case mode&os.ModeSetgid != 0:
		return colorSetgid
	case mode.Perm()&0111 != 0:
		return colorExec
	default:
		return ""
	}
}

// Reset returns the ANSI reset escape sequence.
//
// R2.2 (prd003): clears all ANSI attributes.
// R2.3 (prd003): returns empty string when color is not active.
func Reset() string {
	if !isColorActive() {
		return ""
	}
	return colorReset
}

// ColorEnabled reports whether color output should be used for the given writer.
//
// R2.3 (prd003): checks override first, then whether w is connected to a terminal
// via pkg/sys.IsTerminal.
func ColorEnabled(w io.Writer) bool {
	colorMu.Lock()
	override := colorOverride
	colorMu.Unlock()
	if override == colorForceOn {
		return true
	}
	if override == colorForceOff {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// SetColorEnabled forces color output on or off, overriding terminal detection.
//
// R2.6 (prd003): allows callers to override the default terminal-based decision.
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	defer colorMu.Unlock()
	if enabled {
		colorOverride = colorForceOn
	} else {
		colorOverride = colorForceOff
	}
}

// ResetColorEnabled clears any forced color override, restoring terminal detection.
//
// R2.7 (prd003): undoes a previous SetColorEnabled call.
func ResetColorEnabled() {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = colorAuto
}

// isColorActive returns whether color output is currently active.
// Used by FileTypeColor and Reset to decide whether to emit ANSI codes.
func isColorActive() bool {
	colorMu.Lock()
	defer colorMu.Unlock()
	return colorOverride == colorForceOn
}
