// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"strings"
	"unicode/utf8"
)

// PadRight pads s with trailing spaces to the given width.
// R1.2: width is measured in rune count per R1.3.
func PadRight(s string, width int) string {
	runeLen := utf8.RuneCountInString(s)
	if runeLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-runeLen)
}

// PadLeft pads s with leading spaces to the given width.
// R1.2: width is measured in rune count per R1.3.
func PadLeft(s string, width int) string {
	runeLen := utf8.RuneCountInString(s)
	if runeLen >= width {
		return s
	}
	return strings.Repeat(" ", width-runeLen) + s
}

// columnGap is the minimum spacing between columns, matching GNU ls behavior.
const columnGap = 2

// Columns arranges entries into a columnar grid that fits within termWidth.
// R1.1: distributes entries into the maximum number of columns that fit.
// R1.4: column widths are per-column maximums, not global maximum.
// D3: entries fill top-to-bottom within each column (matching GNU ls).
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}

	// Compute display widths once.
	widths := make([]int, len(entries))
	for i, e := range entries {
		widths[i] = utf8.RuneCountInString(e)
	}

	// Try from max possible columns down to 1.
	n := len(entries)
	for numCols := n; numCols >= 1; numCols-- {
		numRows := (n + numCols - 1) / numCols

		// Compute per-column max widths.
		colWidths := make([]int, numCols)
		for col := range numCols {
			for row := range numRows {
				idx := col*numRows + row
				if idx >= n {
					continue
				}
				if widths[idx] > colWidths[col] {
					colWidths[col] = widths[idx]
				}
			}
		}

		// Check if this layout fits within termWidth.
		totalWidth := 0
		for i, cw := range colWidths {
			totalWidth += cw
			if i < numCols-1 {
				totalWidth += columnGap
			}
		}
		if totalWidth > termWidth && numCols > 1 {
			continue
		}

		// Build the grid.
		result := make([][]string, numRows)
		for row := range numRows {
			var rowEntries []string
			for col := range numCols {
				idx := col*numRows + row
				if idx >= n {
					break
				}
				rowEntries = append(rowEntries, entries[idx])
			}
			result[row] = rowEntries
		}
		return result
	}

	// Fallback: single column.
	result := make([][]string, n)
	for i, e := range entries {
		result[i] = []string{e}
	}
	return result
}
