// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"strings"
	"unicode/utf8"
)

// displayWidth returns the display width of a string in rune count.
// East-asian full-width characters are deferred; rune count is sufficient
// for ASCII-dominant utility output (prd003-format R1.3).
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// PadRight pads s with trailing spaces to reach the specified display width.
// If s is already wider than width, it is returned unchanged.
//
// Implements: prd003-format R1.2
func PadRight(s string, width int) string {
	w := displayWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// PadLeft pads s with leading spaces to reach the specified display width.
// If s is already wider than width, it is returned unchanged.
//
// Implements: prd003-format R1.2
func PadLeft(s string, width int) string {
	w := displayWidth(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}

// Columns distributes entries into the maximum number of columns that fit
// within termWidth. Column widths are computed from the longest entry in each
// column, not the global maximum, matching ls multi-column layout behavior.
// A minimum gap of 2 spaces separates columns.
//
// Returns nil if entries is empty.
//
// Implements: prd003-format R1.1, R1.4
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}

	const columnGap = 2

	// Try from maxCols down to 1 to find the maximum that fits.
	maxCols := len(entries)
	if maxCols < 1 {
		maxCols = 1
	}

	for numCols := maxCols; numCols >= 1; numCols-- {
		numRows := (len(entries) + numCols - 1) / numCols

		// Compute width of each column (longest entry in that column).
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

		// Check if total width fits within termWidth.
		totalWidth := 0
		for i, cw := range colWidths {
			totalWidth += cw
			if i < numCols-1 {
				totalWidth += columnGap
			}
		}

		if totalWidth <= termWidth {
			// Build the grid.
			grid := make([][]string, numRows)
			for row := 0; row < numRows; row++ {
				var rowEntries []string
				for col := 0; col < numCols; col++ {
					idx := col*numRows + row
					if idx >= len(entries) {
						break
					}
					rowEntries = append(rowEntries, entries[idx])
				}
				grid[row] = rowEntries
			}
			return grid
		}
	}

	// Fallback: one entry per row.
	grid := make([][]string, len(entries))
	for i, e := range entries {
		grid[i] = []string{e}
	}
	return grid
}
