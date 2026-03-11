// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd003-format R1.1–R1.4 (column layout: PadRight, PadLeft, Columns).
package format

import "strings"

// PadRight appends trailing spaces to s until len(s) == width.
// Returns s unchanged if len(s) >= width.
// Width is measured in bytes, not runes (prd003-format R1.2, D2).
//
// prd003-format R1.2.
func PadRight(s string, width int) string {
	l := len(s)
	if l >= width {
		return s
	}
	return s + strings.Repeat(" ", width-l)
}

// PadLeft prepends leading spaces to s until len(s) == width.
// Returns s unchanged if len(s) >= width.
// Width is measured in bytes, not runes (prd003-format R1.2, D2).
//
// prd003-format R1.2.
func PadLeft(s string, width int) string {
	l := len(s)
	if l >= width {
		return s
	}
	return strings.Repeat(" ", width-l) + s
}

// Columns arranges entries into the maximum number of columns whose total
// width (column widths plus single-space separators between columns) fits
// within termWidth. It returns a row-major 2D slice; each inner slice is
// one row of display output.
//
// Entries are distributed column-major (top-to-bottom, left-to-right),
// matching GNU ls multi-column layout. Column width is the maximum byte
// length of entries in that column.
//
// Edge cases (prd003-format R1.1, R1.4):
//   - nil or empty entries returns nil.
//   - termWidth <= 0 is treated as a single column.
//   - An entry wider than termWidth still occupies its own row (single column).
//
// prd003-format R1.1, R1.3, R1.4.
func Columns(entries []string, termWidth int) [][]string {
	n := len(entries)
	if n == 0 {
		return nil
	}
	if termWidth <= 0 {
		return singleColumn(entries)
	}

	// Try decreasing column counts until the layout fits within termWidth.
	for ncols := n; ncols >= 1; ncols-- {
		nrows := (n + ncols - 1) / ncols

		// Compute the maximum entry width in each column.
		// Entries fill column-major: entry i is in column (i / nrows).
		colWidths := make([]int, ncols)
		for i, e := range entries {
			col := i / nrows
			if w := len(e); w > colWidths[col] {
				colWidths[col] = w
			}
		}

		// Total display width = sum of column widths + (ncols-1) single-space separators.
		total := ncols - 1
		for _, w := range colWidths {
			total += w
		}

		if total <= termWidth {
			return buildRowMajor(entries, ncols, nrows, colWidths)
		}
	}

	// No multi-column layout fits; fall back to single column.
	return singleColumn(entries)
}

// buildRowMajor constructs the row-major result from entries arranged in
// ncols columns (column-major fill). All columns except the last are padded
// to their computed widths using PadRight so callers can join rows with a
// single space separator.
func buildRowMajor(entries []string, ncols, nrows int, colWidths []int) [][]string {
	result := make([][]string, nrows)
	for r := range result {
		result[r] = make([]string, 0, ncols)
	}

	for i, e := range entries {
		col := i / nrows
		row := i % nrows
		// Pad all columns except the last so joiners need only a single space.
		if col < ncols-1 {
			e = PadRight(e, colWidths[col])
		}
		result[row] = append(result[row], e)
	}

	return result
}

// singleColumn wraps each entry in its own single-element row.
func singleColumn(entries []string) [][]string {
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{e}
	}
	return rows
}
