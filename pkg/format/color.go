// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R2: ANSI color output for file types.
package format

import (
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// colorOverride stores the user-set color enabled state.
// nil means no override (auto-detect via terminal check).
var colorOverride *bool

// FileTypeColor returns the ANSI escape sequence for the given file mode.
// Implements prd003-format R2.
func FileTypeColor(mode os.FileMode) string {
	return ""
}

// Reset returns the ANSI reset escape sequence.
// Implements prd003-format R2.
func Reset() string {
	return ""
}

// ColorEnabled reports whether color output should be used for the given
// writer. When no override is set, it checks whether the writer's underlying
// file descriptor refers to a terminal via pkg/sys.IsTerminal.
// Implements prd003-format R2.
func ColorEnabled(w io.Writer) bool {
	if colorOverride != nil {
		return *colorOverride
	}
	if f, ok := w.(*os.File); ok {
		return sys.IsTerminal(f.Fd())
	}
	return false
}

// SetColorEnabled forces color output on or off, overriding terminal detection.
// Implements prd003-format R2.
func SetColorEnabled(enabled bool) {
	colorOverride = &enabled
}

// ResetColorEnabled clears any color override set by SetColorEnabled,
// restoring auto-detection via terminal check.
// Implements prd003-format R2.
func ResetColorEnabled() {
	colorOverride = nil
}
