// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides shared output formatting for Unix utilities:
// column alignment for tabular displays (ls -C style), ANSI color codes for
// file types, and human-readable size conversion for byte counts.
//
// Implements: prd003-format R1, R2, R3
// Architecture: docs/ARCHITECTURE.yaml (pkg/format/ component)
package format

import (
	"strings"
	"unicode/utf8"
)

// columnSep is the number of spaces inserted between adjacent columns,
// matching the default GNU ls column separator.
const columnSep = 2

// displayWidth returns the display width of s measured in runes, not bytes.
// East-Asian full-width character handling is deferred per prd003-format R1.3.
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// PadRight returns s padded with spaces on the right to reach the given display
// width. Width is measured in runes, not bytes (prd003-format R1.2, R1.3).
// If s is already at or wider than width, s is returned unchanged.
func PadRight(s string, width int) string {
	w := displayWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// PadLeft returns s padded with spaces on the left to reach the given display
// width. Width is measured in runes, not bytes (prd003-format R1.2, R1.3).
// If s is already at or wider than width, s is returned unchanged.
func PadLeft(s string, width int) string {
	w := displayWidth(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}

// Columns distributes entries into the maximum number of columns that fit
// within termWidth characters. Entries fill columns top-to-bottom then
// left-to-right, matching the default GNU ls -C layout. Column widths are
// derived from the longest entry in each column, not the global maximum.
// Adjacent columns are separated by columnSep spaces.
//
// Returns a 2D slice where each inner slice contains the entries for one row,
// ordered left-to-right. The last row may contain fewer entries than the
// others. Returns nil for empty input or non-positive termWidth.
//
// (prd003-format R1.1, R1.4)
func Columns(entries []string, termWidth int) [][]string {
	n := len(entries)
	if n == 0 || termWidth <= 0 {
		return nil
	}

	// Try the maximum column count first, decrement until the layout fits.
	for numCols := n; numCols >= 1; numCols-- {
		numRows := (n + numCols - 1) / numCols
		// effectiveCols is the number of columns actually used; the last
		// column may be partially filled when n is not divisible by numRows.
		effectiveCols := (n + numRows - 1) / numRows

		colWidths := make([]int, effectiveCols)
		for i, e := range entries {
			col := i / numRows
			if w := displayWidth(e); w > colWidths[col] {
				colWidths[col] = w
			}
		}

		totalWidth := 0
		for i, w := range colWidths {
			totalWidth += w
			if i < effectiveCols-1 {
				totalWidth += columnSep
			}
		}

		if totalWidth <= termWidth {
			rows := make([][]string, numRows)
			for r := range rows {
				rows[r] = make([]string, 0, effectiveCols)
			}
			for i, e := range entries {
				row := i % numRows
				rows[row] = append(rows[row], e)
			}
			return rows
		}
	}

	// Single-column fallback: one entry per row regardless of width.
	rows := make([][]string, n)
	for i, e := range entries {
		rows[i] = []string{e}
	}
	return rows
}
