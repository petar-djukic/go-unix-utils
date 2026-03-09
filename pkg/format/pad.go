// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"strings"
	"unicode/utf8"
)

// PadRight pads s with trailing spaces to the given width. R1.2: if s is
// already wider than width, s is returned unchanged. Width is measured in
// rune count per R1.3.
func PadRight(s string, width int) string {
	runeLen := utf8.RuneCountInString(s)
	if runeLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-runeLen)
}

// PadLeft pads s with leading spaces to the given width. R1.2: if s is
// already wider than width, s is returned unchanged. Width is measured in
// rune count per R1.3.
func PadLeft(s string, width int) string {
	runeLen := utf8.RuneCountInString(s)
	if runeLen >= width {
		return s
	}
	return strings.Repeat(" ", width-runeLen) + s
}
