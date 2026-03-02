// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"strings"
	"unicode/utf8"
)

const columnGap = 2 // spaces between columns, matching GNU ls

// displayWidth returns the display width of s in terminal columns. Uses rune
// count, not byte count, so multi-byte UTF-8 characters are measured correctly.
// East-asian full-width handling is deferred per prd003-format R1.3.
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// Columns distributes entries into the maximum number of columns that fit
// within termWidth, using row-major (across) order matching gls --format=across.
// Column widths are computed from the longest entry in each column, not the
// global maximum. Returns nil for empty input.
//
// prd003-format R1.1, R1.4.
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}

	n := len(entries)

	// Try column counts from maximum down to 1.
	for numCols := n; numCols >= 1; numCols-- {
		numRows := (n + numCols - 1) / numCols

		// Calculate per-column widths from entries in row-major order.
		colWidths := make([]int, numCols)
		for i, entry := range entries {
			col := i % numCols
			w := displayWidth(entry)
			if w > colWidths[col] {
				colWidths[col] = w
			}
		}

		// Total width: sum of column widths + gaps between columns.
		totalWidth := 0
		for _, w := range colWidths {
			totalWidth += w
		}
		totalWidth += columnGap * (numCols - 1)

		if totalWidth <= termWidth || numCols == 1 {
			grid := make([][]string, numRows)
			for r := 0; r < numRows; r++ {
				row := make([]string, 0, numCols)
				for c := 0; c < numCols; c++ {
					idx := r*numCols + c
					if idx < n {
						row = append(row, entries[idx])
					}
				}
				grid[r] = row
			}
			return grid
		}
	}

	return nil // unreachable: numCols == 1 always succeeds
}

// PadRight returns s padded with trailing spaces to the specified display
// width. If s is already at or beyond width, it is returned unchanged.
//
// prd003-format R1.2.
func PadRight(s string, width int) string {
	w := displayWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// PadLeft returns s padded with leading spaces to the specified display width.
// If s is already at or beyond width, it is returned unchanged.
//
// prd003-format R1.2.
func PadLeft(s string, width int) string {
	w := displayWidth(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}
