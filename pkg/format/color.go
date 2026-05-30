// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"io"
	"os"
)

// FileTypeColor returns the ANSI escape sequence for a file's type.
func FileTypeColor(mode os.FileMode) string {
	panic("not implemented")
}

// Reset returns the ANSI reset sequence.
func Reset() string {
	panic("not implemented")
}

// ColorEnabled reports whether color output is enabled for the given writer.
func ColorEnabled(w io.Writer) bool {
	panic("not implemented")
}

// SetColorEnabled forces color output on or off regardless of terminal detection.
func SetColorEnabled(enabled bool) {
	panic("not implemented")
}

// ResetColorEnabled clears any color override, restoring terminal-based detection.
func ResetColorEnabled() {
	panic("not implemented")
}
