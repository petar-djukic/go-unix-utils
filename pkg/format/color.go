// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"io"
	"os"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// ANSI escape sequence prefix and suffix used to construct color codes
// (prd003-format R2.1, R2.2).
const (
	ansiEsc   = "\033["
	ansiReset = "\033[0m"
)

// File-type ANSI color codes matching GNU ls LS_COLORS defaults
// (prd003-format R2.4).
const (
	colorDirectory = "34"   // blue
	colorSymlink   = "36"   // cyan
	colorExec      = "32"   // green
	colorBlockDev  = "33;1" // yellow bold
	colorCharDev   = "33;1" // yellow bold
	colorSocket    = "35"   // magenta
	colorPipe      = "33"   // yellow
	colorRegular   = "0"    // reset/default
)

// colorOverride tracks the process-global override set by SetColorEnabled
// (prd003-format R2.6, R2.7).
var (
	colorMu       sync.Mutex
	colorOverride *bool // nil = automatic detection; non-nil = forced value
)

// FileTypeColor returns the ANSI escape sequence for a file's type, matching
// GNU ls LS_COLORS defaults (prd003-format R2.1, R2.4). The returned string
// includes the full escape prefix (e.g., "\033[34m" for directories). Returns
// an empty string when color output is disabled via SetColorEnabled(false) or
// when automatic detection determines the output is not a TTY.
//
// Supported file types: directory, symlink, executable regular file, block
// device, character device, socket, named pipe (FIFO), and regular file.
func FileTypeColor(mode os.FileMode) string {
	if !isColorActive() {
		return ""
	}

	code := fileTypeCode(mode)
	return ansiEsc + code + "m"
}

// fileTypeCode returns the raw ANSI color code string for the given file mode
// (prd003-format R2.4).
func fileTypeCode(mode os.FileMode) string {
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

// Reset returns the ANSI reset sequence ("\033[0m") when color output is
// enabled, or an empty string when disabled (prd003-format R2.2).
func Reset() string {
	if !isColorActive() {
		return ""
	}
	return ansiReset
}

// ColorEnabled reports whether ANSI color output should be used for w. It
// returns true only when w is backed by a TTY file descriptor, as detected by
// pkg/sys.IsTerminal (prd003-format R2.3). If w does not implement a type
// assertion to *os.File, it returns false.
func ColorEnabled(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// SetColorEnabled overrides automatic TTY detection for FileTypeColor and
// Reset. When enabled is true, ANSI sequences are returned regardless of
// whether stdout is a TTY. When false, empty strings are returned regardless.
// The override is process-global (prd003-format R2.6).
func SetColorEnabled(enabled bool) {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = &enabled
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting to
// automatic TTY detection via ColorEnabled (prd003-format R2.7).
func ResetColorEnabled() {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorOverride = nil
}

// isColorActive returns true when color output is enabled. It checks the
// process-global override first; if no override is set, it falls back to
// automatic TTY detection on os.Stdout (prd003-format R2.3, R2.6).
func isColorActive() bool {
	colorMu.Lock()
	override := colorOverride
	colorMu.Unlock()

	if override != nil {
		return *override
	}
	return ColorEnabled(os.Stdout)
}
