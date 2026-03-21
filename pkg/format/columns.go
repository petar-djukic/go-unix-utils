// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R1.1-R1.4: column alignment for tabular output.
package format

import (
	"strings"
	"unicode/utf8"
)

// columnGap is the number of spaces between columns in multi-column layout.
const columnGap = 2

// PadRight pads s with trailing spaces to the given width.
// If s is already at least width runes, it is returned unchanged.
// Width counts runes, not bytes, to handle UTF-8 correctly.
// Implements prd003-format R1.2, R1.3.
func PadRight(s string, width int) string {
	runeLen := utf8.RuneCountInString(s)
	if runeLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-runeLen)
}

// PadLeft pads s with leading spaces to the given width.
// If s is already at least width runes, it is returned unchanged.
// Width counts runes, not bytes, to handle UTF-8 correctly.
// Implements prd003-format R1.2, R1.3.
func PadLeft(s string, width int) string {
	runeLen := utf8.RuneCountInString(s)
	if runeLen >= width {
		return s
	}
	return strings.Repeat(" ", width-runeLen) + s
}

// Columns arranges entries into rows of columns that fit within termWidth.
// Entries are filled column-first (down then across), matching GNU ls default
// output. Returns a slice of rows, where each row is a slice of entry strings.
// Returns nil for empty input or when termWidth <= 0.
// Implements prd003-format R1.1, R1.3, R1.4.
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 || termWidth <= 0 {
		return nil
	}
	if len(entries) == 1 {
		return [][]string{{entries[0]}}
	}
	widths := entryWidths(entries)
	numCols := findMaxColumns(widths, termWidth)
	return buildRows(entries, numCols)
}

// entryWidths returns the rune count for each entry.
func entryWidths(entries []string) []int {
	widths := make([]int, len(entries))
	for i, e := range entries {
		widths[i] = utf8.RuneCountInString(e)
	}
	return widths
}

// findMaxColumns determines the maximum number of columns that fit within
// termWidth using column-first fill order with 2-space gaps between columns.
// Each column's width is the widest entry assigned to it.
func findMaxColumns(widths []int, termWidth int) int {
	n := len(widths)
	for cols := n; cols > 1; cols-- {
		if columnsWidth(widths, cols, n) <= termWidth {
			return cols
		}
	}
	return 1
}

// columnsWidth computes the total display width for a given column count
// using column-first fill. Returns the sum of per-column max widths plus
// inter-column gaps.
func columnsWidth(widths []int, cols, n int) int {
	rows := (n + cols - 1) / cols
	total := 0
	for c := 0; c < cols; c++ {
		maxW := 0
		for r := 0; r < rows; r++ {
			idx := c*rows + r
			if idx < n && widths[idx] > maxW {
				maxW = widths[idx]
			}
		}
		total += maxW
		if c < cols-1 {
			total += columnGap
		}
	}
	return total
}

// buildRows constructs the row-major output from column-first filled entries.
func buildRows(entries []string, numCols int) [][]string {
	n := len(entries)
	rows := (n + numCols - 1) / numCols
	result := make([][]string, rows)
	for r := 0; r < rows; r++ {
		var row []string
		for c := 0; c < numCols; c++ {
			idx := c*rows + r
			if idx < n {
				row = append(row, entries[idx])
			}
		}
		result[r] = row
	}
	return result
}
