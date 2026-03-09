// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Column alignment padding functions for tabular output.
// Implements prd003-format R1.2 (PadRight, PadLeft).
package format

import "strings"

// PadRight right-pads s with spaces to the given width. Returns s unchanged
// if len(s) >= width. (prd003-format R1.2)
func PadRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// PadLeft left-pads s with spaces to the given width. Returns s unchanged
// if len(s) >= width. (prd003-format R1.2)
func PadLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}
