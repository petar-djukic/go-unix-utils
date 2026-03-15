// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R2.5-R2.7: PadRight, PadLeft, and Columns column alignment.
// R2.5: PadRight right-pads a string with spaces to a given width.
// R2.6: PadLeft left-pads a string with spaces to a given width.
// R2.7: Columns arranges entries into a column-first grid fitting termWidth.

package format

import (
	"strings"
	"unicode/utf8"
)

// PadRight right-pads s with spaces to the given width. If the display width
// of s is already at or beyond width, s is returned unchanged.
//
// R1.2: Right-aligned field padding within a fixed-width column.
// R1.3: Uses rune count for display width, not byte count.
func PadRight(s string, width int) string {
	displayWidth := utf8.RuneCountInString(s)
	if displayWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-displayWidth)
}

// PadLeft left-pads s with spaces to the given width. If the display width
// of s is already at or beyond width, s is returned unchanged.
//
// R1.2: Left-aligned field padding within a fixed-width column.
// R1.3: Uses rune count for display width, not byte count.
func PadLeft(s string, width int) string {
	displayWidth := utf8.RuneCountInString(s)
	if displayWidth >= width {
		return s
	}
	return strings.Repeat(" ", width-displayWidth) + s
}

// Columns arranges entries into a column-first grid that fits within
// termWidth. Entries fill top-to-bottom, left-to-right, matching the GNU ls
// default column layout. The function chooses the maximum number of columns
// such that all columns fit within termWidth with at least two spaces between
// adjacent columns. Each inner slice is one row of the table.
//
// Returns nil if entries is empty. Falls back to a single column if termWidth
// is too narrow for multiple columns.
//
// R1.1: Maximum columns that fit within termWidth, per-column widths.
// R1.3: Uses rune count for display width.
// R1.4: Column-first fill matching ls default output.
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}

	// Precompute display widths.
	widths := make([]int, len(entries))
	for i, e := range entries {
		widths[i] = utf8.RuneCountInString(e)
	}

	// minGap is the minimum number of spaces between adjacent columns.
	const minGap = 2

	// Try increasing column counts from len(entries) down to 1 to find the
	// maximum number of columns that fits.
	bestCols := 1
	for numCols := len(entries); numCols > 1; numCols-- {
		numRows := (len(entries) + numCols - 1) / numCols

		// Compute the maximum display width of each column.
		totalWidth := 0
		fits := true
		for col := range numCols {
			colMax := 0
			for row := range numRows {
				idx := col*numRows + row
				if idx >= len(entries) {
					continue
				}
				if widths[idx] > colMax {
					colMax = widths[idx]
				}
			}
			totalWidth += colMax
			if col < numCols-1 {
				totalWidth += minGap
			}
			if totalWidth > termWidth {
				fits = false
				break
			}
		}
		if fits {
			bestCols = numCols
			break
		}
	}

	numRows := (len(entries) + bestCols - 1) / bestCols

	// Build the result grid row by row.
	result := make([][]string, numRows)
	for row := range numRows {
		var rowEntries []string
		for col := range bestCols {
			idx := col*numRows + row
			if idx >= len(entries) {
				break
			}
			rowEntries = append(rowEntries, entries[idx])
		}
		result[row] = rowEntries
	}

	return result
}
