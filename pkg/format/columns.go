// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// columns.go provides column alignment for tabular output (ls -l style),
// accepting output width as a parameter. The caller obtains terminal width
// from pkg/sys or other sources (design decision D1).
//
// Implements: prd003-format R1.1, R1.2, R1.3, R1.4.
package format

import (
	"strings"
	"unicode/utf8"
)

// columnGap is the minimum number of spaces between columns.
const columnGap = 2

// Columns distributes entries into the maximum number of columns that fit
// within termWidth. Column widths are computed from the longest entry in each
// column, not the global maximum (prd003-format R1.1, R1.4).
//
// Returns a slice of rows, where each row is a slice of entries. Returns nil
// for empty input.
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}

	if termWidth <= 0 {
		termWidth = 80
	}

	// Try increasing column counts from the maximum possible down to 1.
	// The maximum number of columns is bounded by the entry count.
	maxCols := len(entries)

	for numCols := maxCols; numCols >= 1; numCols-- {
		numRows := (len(entries) + numCols - 1) / numCols

		// Compute per-column widths based on the longest entry in each column.
		// Entries are laid out column-major (down then across), matching GNU ls.
		colWidths := make([]int, numCols)
		for col := 0; col < numCols; col++ {
			for row := 0; row < numRows; row++ {
				idx := col*numRows + row
				if idx >= len(entries) {
					continue
				}
				w := displayWidth(entries[idx])
				if w > colWidths[col] {
					colWidths[col] = w
				}
			}
		}

		// Check if this column layout fits within termWidth.
		totalWidth := 0
		for i, w := range colWidths {
			totalWidth += w
			if i < numCols-1 {
				totalWidth += columnGap
			}
		}

		if totalWidth <= termWidth {
			// Build the result grid.
			result := make([][]string, numRows)
			for row := 0; row < numRows; row++ {
				var rowEntries []string
				for col := 0; col < numCols; col++ {
					idx := col*numRows + row
					if idx >= len(entries) {
						continue
					}
					rowEntries = append(rowEntries, entries[idx])
				}
				result[row] = rowEntries
			}
			return result
		}
	}

	// Fallback: one entry per row.
	result := make([][]string, len(entries))
	for i, e := range entries {
		result[i] = []string{e}
	}
	return result
}

// PadRight returns s padded with trailing spaces to the specified display
// width. If s is already at least width runes wide, it is returned unchanged.
//
// Implements: prd003-format R1.2.
func PadRight(s string, width int) string {
	w := displayWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// PadLeft returns s padded with leading spaces to the specified display
// width. If s is already at least width runes wide, it is returned unchanged.
//
// Implements: prd003-format R1.2.
func PadLeft(s string, width int) string {
	w := displayWidth(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}

// displayWidth returns the display width of s in columns. Uses rune count,
// not byte count, per prd003-format R1.3. East-asian full-width characters
// are not handled in the current roadmap; rune count is sufficient.
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}
