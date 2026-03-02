// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd003-format (R1)
package format

import (
	"strings"
	"unicode/utf8"
)

// columnSeparatorWidth is the number of spaces inserted between adjacent columns
// in tabular output, matching GNU ls column spacing. (prd003-format R1.1)
const columnSeparatorWidth = 2

// displayWidth returns the display width of s in terminal columns, computed as
// the rune count. East-asian full-width character handling is deferred; rune
// count is sufficient for ASCII-dominant utility output in the current roadmap.
// (prd003-format R1.3)
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// PadRight returns s padded with trailing spaces to the specified display width.
// If s is already at least width runes wide, it is returned unchanged. Width is
// measured in runes, not bytes, to handle multi-byte UTF-8 correctly.
// (prd003-format R1.2, R1.3)
func PadRight(s string, width int) string {
	dw := displayWidth(s)
	if dw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-dw)
}

// PadLeft returns s padded with leading spaces to the specified display width.
// If s is already at least width runes wide, it is returned unchanged. Width is
// measured in runes, not bytes, to handle multi-byte UTF-8 correctly.
// (prd003-format R1.2, R1.3)
func PadLeft(s string, width int) string {
	dw := displayWidth(s)
	if dw >= width {
		return s
	}
	return strings.Repeat(" ", width-dw) + s
}

// Columns distributes entries into the maximum number of columns that fit within
// termWidth, using the longest entry in each column to determine column width.
// Entries are filled row-by-row (across layout, matching gls --format=across).
// Adjacent columns are separated by two spaces. Returns nil for empty input.
// (prd003-format R1.1, R1.4)
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}

	// Pre-compute display widths for all entries. (prd003-format R1.3)
	widths := make([]int, len(entries))
	for i, e := range entries {
		widths[i] = displayWidth(e)
	}

	// Single entry: one column, one row.
	if len(entries) == 1 {
		return [][]string{{entries[0]}}
	}

	// Upper bound on column count: each column needs at least 1 character, and
	// all but the last column need a 2-space separator.
	maxCols := len(entries)
	upperBound := (termWidth + columnSeparatorWidth) / (1 + columnSeparatorWidth)
	if maxCols > upperBound {
		maxCols = upperBound
	}
	if maxCols < 1 {
		maxCols = 1
	}

	// Try from maximum possible columns down to 1. The first (highest) column
	// count whose total width fits within termWidth is optimal. (prd003-format R1.1)
	bestCols := 1
	for nCols := maxCols; nCols >= 2; nCols-- {
		colWidths := columnWidths(widths, nCols)
		total := totalWidth(colWidths)
		if total <= termWidth {
			bestCols = nCols
			break
		}
	}

	// Build the result grid with across (row-by-row) distribution.
	nRows := (len(entries) + bestCols - 1) / bestCols
	result := make([][]string, nRows)
	for r := 0; r < nRows; r++ {
		row := make([]string, 0, bestCols)
		for c := 0; c < bestCols; c++ {
			idx := r*bestCols + c
			if idx < len(entries) {
				row = append(row, entries[idx])
			}
		}
		result[r] = row
	}
	return result
}

// columnWidths computes the maximum display width of entries in each column
// for an across (row-by-row) distribution with nCols columns. (prd003-format R1.1)
func columnWidths(widths []int, nCols int) []int {
	colW := make([]int, nCols)
	for i, w := range widths {
		col := i % nCols
		if w > colW[col] {
			colW[col] = w
		}
	}
	return colW
}

// totalWidth returns the total display width of a column layout: the sum of all
// column widths plus two-space separators between adjacent columns.
// (prd003-format R1.1)
func totalWidth(colWidths []int) int {
	total := 0
	for i, w := range colWidths {
		total += w
		if i < len(colWidths)-1 {
			total += columnSeparatorWidth
		}
	}
	return total
}
