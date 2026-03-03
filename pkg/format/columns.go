// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"strings"
	"unicode/utf8"
)

// columnGap is the number of spaces between adjacent columns in grid output.
const columnGap = 2

// Columns distributes entries into the maximum number of columns that fit
// within termWidth, using the longest entry in each column to determine column
// width (prd003-format R1.1, R1.4). Entries are ordered column-major: they
// fill down the first column, then the second, matching GNU ls default layout.
// Returns nil for empty input.
func Columns(entries []string, termWidth int) [][]string {
	n := len(entries)
	if n == 0 {
		return nil
	}

	bestCols := 1

	for numCols := n; numCols >= 1; numCols-- {
		numRows := (n + numCols - 1) / numCols

		colWidths := make([]int, numCols)
		for i, entry := range entries {
			col := i / numRows
			w := utf8.RuneCountInString(entry)
			if w > colWidths[col] {
				colWidths[col] = w
			}
		}

		total := 0
		for _, w := range colWidths {
			total += w
		}
		if numCols > 1 {
			total += columnGap * (numCols - 1)
		}

		if total <= termWidth {
			bestCols = numCols
			break
		}
	}

	numRows := (n + bestCols - 1) / bestCols
	rows := make([][]string, numRows)
	for r := 0; r < numRows; r++ {
		row := make([]string, 0, bestCols)
		for c := 0; c < bestCols; c++ {
			idx := c*numRows + r
			if idx < n {
				row = append(row, entries[idx])
			}
		}
		rows[r] = row
	}
	return rows
}

// PadRight pads s with trailing spaces to the specified display width using
// rune count (prd003-format R1.2, R1.3). If s is already at least width runes
// wide, it is returned unchanged.
func PadRight(s string, width int) string {
	runeCount := utf8.RuneCountInString(s)
	if runeCount >= width {
		return s
	}
	return s + strings.Repeat(" ", width-runeCount)
}

// PadLeft pads s with leading spaces to the specified display width using
// rune count (prd003-format R1.2, R1.3). If s is already at least width runes
// wide, it is returned unchanged.
func PadLeft(s string, width int) string {
	runeCount := utf8.RuneCountInString(s)
	if runeCount >= width {
		return s
	}
	return strings.Repeat(" ", width-runeCount) + s
}
