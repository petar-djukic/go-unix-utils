// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// ANSI escape sequences for GNU ls LS_COLORS default color assignments.
// Per prd003-format R2.4.
const (
	colorDirectory = "\033[34m"  // blue
	colorSymlink   = "\033[36m"  // cyan
	colorExec      = "\033[32m"  // green
	colorBlockDev  = "\033[33;1m" // yellow bold
	colorCharDev   = "\033[33;1m" // yellow bold
	colorSocket    = "\033[35m"  // magenta
	colorPipe      = "\033[33m"  // yellow
	ansiReset      = "\033[0m"
)

// colorEnabled tracks whether color output is active for the current process.
// Set via EnableColor/DisableColor or queried via ColorEnabled.
var colorEnabled *bool

// ColorEnabled returns true only when w is a TTY. Delegates the TTY check to
// pkg/sys.IsTerminal (prd002-sys R1.3). When false, FileTypeColor returns an
// empty string and Reset returns an empty string.
//
// Per prd003-format R2.3.
// Design decision D1: must not import golang.org/x/term.
// Design decision D5: type-asserts to *os.File to extract the fd.
func ColorEnabled(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return sys.IsTerminal(f.Fd())
}

// SetColorEnabled overrides the automatic TTY detection. This is useful for
// --color=always and --color=never flags in utilities like ls.
func SetColorEnabled(enabled bool) {
	colorEnabled = &enabled
}

// ResetColorEnabled clears any override set by SetColorEnabled, reverting to
// automatic TTY detection via ColorEnabled.
func ResetColorEnabled() {
	colorEnabled = nil
}

// isColorActive returns true when color output should be produced. If an
// override has been set via SetColorEnabled, it returns that value. Otherwise
// it checks stdout via ColorEnabled.
func isColorActive() bool {
	if colorEnabled != nil {
		return *colorEnabled
	}
	return ColorEnabled(os.Stdout)
}

// FileTypeColor returns the ANSI escape sequence for a file's type. Returns
// an empty string for regular files or when color is not enabled.
//
// Per prd003-format R2.1, R2.4.
// Utility context: ls outputs ANSI escapes before each filename in colorized
// mode (ls.c:103, ls.c:273).
func FileTypeColor(mode os.FileMode) string {
	if !isColorActive() {
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
	case mode&os.ModeType == 0 && mode&0o111 != 0:
		// Regular file with at least one execute bit set.
		return colorExec
	default:
		// Regular file with no execute bits.
		return ""
	}
}

// Reset returns the ANSI reset sequence ("\033[0m") when color is enabled, or
// an empty string when color is not enabled.
//
// Per prd003-format R2.2.
func Reset() string {
	if !isColorActive() {
		return ""
	}
	return ansiReset
}
