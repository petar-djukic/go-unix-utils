// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// columns.go implements prd003 R1.1, R1.3, R1.4: multi-column layout for ls-style output.

package format

import "unicode/utf8"

const columnGap = 2

// Columns arranges entries into the maximum number of columns that fit within
// termWidth. Entries fill top-to-bottom within each column, then left-to-right
// across columns, matching GNU ls default multi-column layout.
//
// R1.1 (prd003): distributes entries into columns fitting termWidth.
// R1.3 (prd003): uses rune count for display width.
// R1.4 (prd003): per-column widths from longest entry in each column.
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}
	widths := entryWidths(entries)
	numCols := findMaxColumns(widths, termWidth)
	return buildRows(entries, numCols)
}

// entryWidths returns the display width (rune count) of each entry.
func entryWidths(entries []string) []int {
	w := make([]int, len(entries))
	for i, e := range entries {
		w[i] = utf8.RuneCountInString(e)
	}
	return w
}

// findMaxColumns determines the maximum number of columns whose per-column
// widths (plus gaps) fit within termWidth.
func findMaxColumns(widths []int, termWidth int) int {
	n := len(widths)
	best := 1
	for cols := n; cols > 1; cols-- {
		if columnsFit(widths, cols, termWidth) {
			best = cols
			break
		}
	}
	return best
}

// columnsFit checks whether distributing len(widths) entries into cols columns
// (top-to-bottom fill) fits within termWidth.
func columnsFit(widths []int, cols, termWidth int) bool {
	n := len(widths)
	rows := (n + cols - 1) / cols
	total := 0
	for c := range cols {
		maxW := 0
		for r := range rows {
			idx := c*rows + r
			if idx < n && widths[idx] > maxW {
				maxW = widths[idx]
			}
		}
		if c < cols-1 {
			total += maxW + columnGap
		} else {
			total += maxW
		}
		if total > termWidth {
			return false
		}
	}
	return true
}

// buildRows constructs the row-oriented output from a column-major fill order.
func buildRows(entries []string, numCols int) [][]string {
	n := len(entries)
	rows := (n + numCols - 1) / numCols
	result := make([][]string, rows)
	for r := range rows {
		var row []string
		for c := range numCols {
			idx := c*rows + r
			if idx < n {
				row = append(row, entries[idx])
			}
		}
		result[r] = row
	}
	return result
}
