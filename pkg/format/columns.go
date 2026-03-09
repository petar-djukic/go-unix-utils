// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "unicode/utf8"

// columnGap is the number of spaces between columns in multi-column output.
const columnGap = 2

// Columns arranges entries into columns that fit within termWidth, filling
// column-first (top to bottom, left to right) like GNU ls default output.
// R1.1, R1.4: determines the maximum number of columns that fit, accounting
// for a two-space gap between columns. Returns a slice of rows, where each
// row is a slice of entries.
func Columns(entries []string, termWidth int) [][]string {
	n := len(entries)
	if n == 0 {
		return nil
	}

	// Try decreasing column counts from max possible down to 1.
	maxCols := min(max(termWidth/2, 1), n)

	for numCols := maxCols; numCols > 1; numCols-- {
		numRows := (n + numCols - 1) / numCols
		colWidths := computeColumnWidths(entries, numCols, numRows, n)
		totalWidth := totalColumnsWidth(colWidths)
		if totalWidth <= termWidth {
			return buildRows(entries, numCols, numRows, n)
		}
	}

	// Fallback: one column per row.
	rows := make([][]string, n)
	for i, e := range entries {
		rows[i] = []string{e}
	}
	return rows
}

// computeColumnWidths returns the width of each column given column-first fill order.
func computeColumnWidths(entries []string, numCols, numRows, n int) []int {
	widths := make([]int, numCols)
	for col := range numCols {
		for row := range numRows {
			idx := col*numRows + row
			if idx >= n {
				continue
			}
			w := utf8.RuneCountInString(entries[idx])
			if w > widths[col] {
				widths[col] = w
			}
		}
	}
	return widths
}

// totalColumnsWidth computes the total display width including inter-column gaps.
func totalColumnsWidth(colWidths []int) int {
	total := 0
	for i, w := range colWidths {
		total += w
		if i < len(colWidths)-1 {
			total += columnGap
		}
	}
	return total
}

// buildRows constructs the output rows from column-first ordering.
func buildRows(entries []string, numCols, numRows, n int) [][]string {
	rows := make([][]string, numRows)
	for row := range numRows {
		var cols []string
		for col := range numCols {
			idx := col*numRows + row
			if idx >= n {
				break
			}
			cols = append(cols, entries[idx])
		}
		rows[row] = cols
	}
	return rows
}
