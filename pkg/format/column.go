// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

// PadRight pads s with trailing spaces to the given width.
// R3.1: returns s unchanged if len(s) >= width.
func PadRight(s string, width int) string {
	panic("not implemented")
}

// PadLeft pads s with leading spaces to the given width.
// R3.2: returns s unchanged if len(s) >= width.
func PadLeft(s string, width int) string {
	panic("not implemented")
}

// Columns arranges entries into a grid of rows and columns that fits within
// termWidth characters.
// R3.3: returns a slice of rows, each row being a slice of column entries.
func Columns(entries []string, termWidth int) [][]string {
	panic("not implemented")
}
