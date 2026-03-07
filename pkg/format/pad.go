// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"strings"
	"unicode/utf8"
)

// PadRight pads s with trailing spaces to the given display width. If s is
// already at or beyond width (by rune count), it is returned unchanged.
// (prd003-format R1.2, R1.3)
func PadRight(s string, width int) string {
	runeLen := utf8.RuneCountInString(s)
	if runeLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-runeLen)
}

// PadLeft pads s with leading spaces to the given display width. If s is
// already at or beyond width (by rune count), it is returned unchanged.
// (prd003-format R1.2, R1.3)
func PadLeft(s string, width int) string {
	runeLen := utf8.RuneCountInString(s)
	if runeLen >= width {
		return s
	}
	return strings.Repeat(" ", width-runeLen) + s
}
