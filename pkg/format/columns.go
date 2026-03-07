// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "unicode/utf8"

// columnGap is the minimum number of spaces between columns, matching GNU ls.
const columnGap = 2

// Columns arranges entries into columns that fit within termWidth. Entries are
// distributed top-to-bottom then left-to-right, matching GNU ls column order.
// Column widths are computed from the longest entry in each column (R1.4).
// Returns a slice of rows, where each row is a slice of entries.
// (prd003-format R1.1, R1.3, R1.4)
func Columns(entries []string, termWidth int) [][]string {
	n := len(entries)
	if n == 0 {
		return nil
	}

	// Try to maximize column count, starting from the most columns possible.
	for numCols := n; numCols >= 1; numCols-- {
		numRows := (n + numCols - 1) / numCols

		// Compute per-column widths from the longest entry in each column.
		colWidths := make([]int, numCols)
		for col := range numCols {
			for row := range numRows {
				idx := col*numRows + row
				if idx >= n {
					continue
				}
				w := utf8.RuneCountInString(entries[idx])
				if w > colWidths[col] {
					colWidths[col] = w
				}
			}
		}

		// Check if the columns fit within termWidth.
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

		// Build the row-major result.
		rows := make([][]string, numRows)
		for row := range numRows {
			var rowEntries []string
			for col := range numCols {
				idx := col*numRows + row
				if idx >= n {
					break
				}
				rowEntries = append(rowEntries, entries[idx])
			}
			rows[row] = rowEntries
		}
		return rows
	}

	// Fallback: single column (unreachable since loop includes numCols=1).
	rows := make([][]string, n)
	for i, e := range entries {
		rows[i] = []string{e}
	}
	return rows
}
