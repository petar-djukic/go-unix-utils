// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"io"
	"os"
)

// FileTypeColor returns the ANSI escape sequence for the given file mode.
//
// R2.1 (prd003): maps file type (directory, symlink, executable, etc.) to color codes.
func FileTypeColor(mode os.FileMode) string {
	return ""
}

// Reset returns the ANSI reset escape sequence.
//
// R2.2 (prd003): clears all ANSI attributes.
func Reset() string {
	return ""
}

// ColorEnabled reports whether color output should be used for the given writer.
//
// R2.3 (prd003): checks whether w is connected to a terminal.
func ColorEnabled(w io.Writer) bool {
	return false
}

// SetColorEnabled forces color output on or off, overriding terminal detection.
//
// R2.4 (prd003): allows callers to override the default terminal-based decision.
func SetColorEnabled(enabled bool) {
}

// ResetColorEnabled clears any forced color override, restoring terminal detection.
//
// R2.5 (prd003): undoes a previous SetColorEnabled call.
func ResetColorEnabled() {
}
