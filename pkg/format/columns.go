// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "unicode/utf8"

// PadRight pads s with trailing spaces to the given display width.
func PadRight(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	buf := make([]byte, 0, len(s)+width-n)
	buf = append(buf, s...)
	for range width - n {
		buf = append(buf, ' ')
	}
	return string(buf)
}

// PadLeft pads s with leading spaces to the given display width.
func PadLeft(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	buf := make([]byte, 0, len(s)+width-n)
	for range width - n {
		buf = append(buf, ' ')
	}
	buf = append(buf, s...)
	return string(buf)
}

// Columns arranges entries into a grid that fits within termWidth columns.
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}

	widths := make([]int, len(entries))
	for i, e := range entries {
		widths[i] = utf8.RuneCountInString(e)
	}

	for numCols := len(entries); numCols >= 1; numCols-- {
		numRows := (len(entries) + numCols - 1) / numCols

		colWidths := make([]int, numCols)
		for i, w := range widths {
			col := i / numRows
			if w > colWidths[col] {
				colWidths[col] = w
			}
		}

		total := 0
		for i, cw := range colWidths {
			total += cw
			if i < numCols-1 {
				total += 2
			}
		}

		if total <= termWidth {
			rows := make([][]string, numRows)
			for r := range numRows {
				row := make([]string, 0, numCols)
				for c := range numCols {
					idx := c*numRows + r
					if idx < len(entries) {
						row = append(row, entries[idx])
					}
				}
				rows[r] = row
			}
			return rows
		}
	}

	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{e}
	}
	return rows
}
