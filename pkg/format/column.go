// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"strings"
	"unicode/utf8"
)

// columnGap is the number of spaces inserted between adjacent columns.
const columnGap = 2

// Columns distributes entries into the maximum number of columns that fit
// within termWidth characters. Entries are placed in row-major order (left to
// right, top to bottom). Each column width is determined by the longest entry
// in that column, not the global maximum.
// R1.1, R1.4: multi-column layout matching ls column-layout behavior.
func Columns(entries []string, termWidth int) [][]string {
	n := len(entries)
	if n == 0 {
		return nil
	}

	// Try from n columns down to 2; single-column is the fallback.
	for numCols := n; numCols > 1; numCols-- {
		numRows := (n + numCols - 1) / numCols

		// R1.4: compute per-column max display widths using row-major distribution.
		colWidths := make([]int, numCols)
		for i, entry := range entries {
			col := i % numCols
			w := displayWidth(entry)
			if w > colWidths[col] {
				colWidths[col] = w
			}
		}

		// Total width including gaps between columns.
		total := 0
		for c, cw := range colWidths {
			total += cw
			if c < numCols-1 {
				total += columnGap
			}
		}

		if total <= termWidth {
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

	// Single-column fallback: one entry per row.
	grid := make([][]string, n)
	for i, e := range entries {
		grid[i] = []string{e}
	}
	return grid
}

// PadRight returns s padded with trailing spaces to the specified display width.
// If s is already at least width runes wide, it is returned unchanged.
// R1.2: left-aligned field padding within a fixed-width column.
func PadRight(s string, width int) string {
	dw := displayWidth(s)
	if dw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-dw)
}

// PadLeft returns s padded with leading spaces to the specified display width.
// If s is already at least width runes wide, it is returned unchanged.
// R1.2: right-aligned field padding within a fixed-width column.
func PadLeft(s string, width int) string {
	dw := displayWidth(s)
	if dw >= width {
		return s
	}
	return strings.Repeat(" ", width-dw) + s
}

// displayWidth returns the display width of s in terminal columns, computed as
// the rune count. East-Asian full-width character handling is deferred per
// prd003-format; rune count is sufficient for ASCII-dominant utility output.
// R1.3: display width via rune count, not byte count.
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}
