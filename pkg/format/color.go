// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"io"
	"os"
)

// FileTypeColor returns the ANSI escape sequence for the given file mode.
// R2.1: maps file types (directory, symlink, executable, etc.) to color codes.
func FileTypeColor(mode os.FileMode) string {
	return ""
}

// Reset returns the ANSI reset escape sequence.
// R2.2: resets terminal color to default after colored output.
func Reset() string {
	return ""
}

// ColorEnabled reports whether color output is enabled for the given writer.
// R2.3: checks whether w is connected to a terminal via pkg/sys.
func ColorEnabled(w io.Writer) bool {
	return false
}

// SetColorEnabled forces color output on or off regardless of terminal detection.
// R2.4: overrides the automatic terminal detection.
func SetColorEnabled(enabled bool) {
}

// ResetColorEnabled clears the forced color override set by SetColorEnabled.
// R2.5: returns to automatic terminal detection behavior.
func ResetColorEnabled() {
}
