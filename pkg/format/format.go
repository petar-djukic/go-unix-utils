// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// human-readable size conversion, ANSI color output, column alignment, and
// string padding.
//
// Implements: srd003-format
package format

import (
	"io"
	"os"
)

// HumanSizeOpts configures human-readable size formatting.
type HumanSizeOpts struct {
	Binary bool
}

// HumanSize formats a byte count as a human-readable string with unit suffixes.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	return ""
}

// FileTypeColor returns the ANSI color escape sequence for the given file mode.
func FileTypeColor(mode os.FileMode) string {
	return ""
}

// Reset returns the ANSI reset escape sequence.
func Reset() string {
	return ""
}

// ColorEnabled reports whether color output is enabled for the given writer.
func ColorEnabled(w io.Writer) bool {
	return false
}

// SetColorEnabled overrides the automatic color detection with the given value.
func SetColorEnabled(enabled bool) {
}

// ResetColorEnabled clears any manual color override, restoring automatic detection.
func ResetColorEnabled() {
}

// PadRight pads s on the right with spaces to the given width.
func PadRight(s string, width int) string {
	return ""
}

// PadLeft pads s on the left with spaces to the given width.
func PadLeft(s string, width int) string {
	return ""
}

// Columns arranges entries into columns that fit within termWidth.
func Columns(entries []string, termWidth int) [][]string {
	return nil
}
