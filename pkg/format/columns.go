// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Columns layout function for multi-column tabular output.
// Implements prd003-format R1.1, R1.3, R1.4 (column alignment).
package format

import "unicode/utf8"

// columnGap is the number of spaces between columns, matching GNU ls behavior.
const columnGap = 2

// Columns distributes entries into the maximum number of columns that fit
// within termWidth, using column-major fill order (top-to-bottom within each
// column, then left-to-right across columns), matching GNU ls default layout.
// Column widths are computed from the longest entry in each column, not the
// global maximum. Returns a slice of rows, where each row is a slice of
// entries. (prd003-format R1.1, R1.3, R1.4)
func Columns(entries []string, termWidth int) [][]string {
	n := len(entries)
	// R3.4: empty input returns nil.
	if n == 0 {
		return nil
	}

	// R3.4: single entry returns one column.
	if n == 1 {
		return [][]string{{entries[0]}}
	}

	// Precompute display widths using rune count (R1.3).
	widths := make([]int, n)
	for i, e := range entries {
		widths[i] = utf8.RuneCountInString(e)
	}

	// Try increasing column counts to find the maximum that fits.
	bestCols := 1
	for cols := 2; cols <= n; cols++ {
		rows := (n + cols - 1) / cols
		if fitsInWidth(widths, n, rows, cols, termWidth) {
			bestCols = cols
		}
	}

	return buildLayout(entries, n, bestCols)
}

// fitsInWidth checks whether a layout with the given rows and cols fits
// within termWidth. Column widths are determined by the widest entry in
// each column (R1.4), with columnGap spaces between columns.
func fitsInWidth(widths []int, n, rows, cols, termWidth int) bool {
	totalWidth := 0
	for col := range cols {
		maxW := 0
		for row := range rows {
			idx := col*rows + row
			if idx >= n {
				continue
			}
			if widths[idx] > maxW {
				maxW = widths[idx]
			}
		}
		totalWidth += maxW
		// Add gap between columns (not after the last column).
		if col < cols-1 {
			totalWidth += columnGap
		}
	}
	return totalWidth <= termWidth
}

// buildLayout arranges entries into rows using column-major order with the
// given number of columns.
func buildLayout(entries []string, n, cols int) [][]string {
	rows := (n + cols - 1) / cols
	result := make([][]string, rows)
	for row := range rows {
		var rowEntries []string
		for col := range cols {
			idx := col*rows + row
			if idx >= n {
				break
			}
			rowEntries = append(rowEntries, entries[idx])
		}
		result[row] = rowEntries
	}
	return result
}
