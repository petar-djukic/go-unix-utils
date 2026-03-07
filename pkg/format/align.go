// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"strings"
	"unicode/utf8"
)

// columnGap is the minimum number of spaces between columns.
const columnGap = 2

// PadRight pads s with trailing spaces to the specified width. R1.2.
// If s is already at or beyond width (by rune count), it is returned unchanged.
func PadRight(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

// PadLeft pads s with leading spaces to the specified width. R1.2.
// If s is already at or beyond width (by rune count), it is returned unchanged.
func PadLeft(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return strings.Repeat(" ", width-n) + s
}

// Columns arranges entries into columns for terminal display. R1.1, R1.3, R1.4.
//
// Entries are filled top-to-bottom within each column, left-to-right across
// columns, matching GNU ls default column layout. The number of columns is the
// maximum that fits within termWidth with at least columnGap spaces between
// columns. Column widths are determined by the longest entry in each column.
//
// Returns nil if entries is empty. Returns a single column if nothing fits in
// multiple columns.
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}

	// Compute display widths once.
	widths := make([]int, len(entries))
	for i, e := range entries {
		widths[i] = utf8.RuneCountInString(e)
	}

	// Try increasing column counts until the layout no longer fits.
	bestCols := 1
	for numCols := 2; numCols <= len(entries); numCols++ {
		numRows := (len(entries) + numCols - 1) / numCols
		totalWidth := 0
		fits := true
		for col := 0; col < numCols; col++ {
			maxW := 0
			for row := 0; row < numRows; row++ {
				idx := col*numRows + row
				if idx < len(widths) && widths[idx] > maxW {
					maxW = widths[idx]
				}
			}
			if col < numCols-1 {
				totalWidth += maxW + columnGap
			} else {
				totalWidth += maxW
			}
			if totalWidth > termWidth {
				fits = false
				break
			}
		}
		if fits {
			bestCols = numCols
		} else {
			break
		}
	}

	numRows := (len(entries) + bestCols - 1) / bestCols
	result := make([][]string, numRows)
	for row := 0; row < numRows; row++ {
		var rowEntries []string
		for col := 0; col < bestCols; col++ {
			idx := col*numRows + row
			if idx < len(entries) {
				rowEntries = append(rowEntries, entries[idx])
			}
		}
		result[row] = rowEntries
	}
	return result
}
