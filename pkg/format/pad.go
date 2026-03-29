// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// pad.go implements prd003 R1.2: pad functions for column alignment.

package format

import "unicode/utf8"

// PadRight pads s with trailing spaces to the given width based on rune count.
// If s is already at or beyond width, it is returned unchanged (no truncation).
//
// R1.2 (prd003): right-pads for left-aligned column output.
func PadRight(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	pad := width - n
	buf := make([]byte, 0, len(s)+pad)
	buf = append(buf, s...)
	for range pad {
		buf = append(buf, ' ')
	}
	return string(buf)
}

// PadLeft pads s with leading spaces to the given width based on rune count.
// If s is already at or beyond width, it is returned unchanged (no truncation).
//
// R1.2 (prd003): left-pads for right-aligned column output.
func PadLeft(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	pad := width - n
	buf := make([]byte, 0, len(s)+pad)
	for range pad {
		buf = append(buf, ' ')
	}
	buf = append(buf, s...)
	return string(buf)
}
