// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"strings"
	"unicode/utf8"
)

const columnGap = 2

// PadRight pads s with trailing spaces to the given width, measured in runes.
// R4.1: used for left-aligned column output.
func PadRight(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

// PadLeft pads s with leading spaces to the given width, measured in runes.
// R4.2: used for right-aligned numeric columns.
func PadLeft(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return strings.Repeat(" ", width-n) + s
}

// Columns arranges entries into a grid that fits within termWidth columns.
// R4.3: fills top-to-bottom, left-to-right like GNU ls -C with a 2-space gap.
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}
	numCols := findMaxColumns(entries, termWidth)
	return buildGrid(entries, numCols)
}

func findMaxColumns(entries []string, termWidth int) int {
	n := len(entries)
	for cols := n; cols > 1; cols-- {
		if gridFits(entries, cols, termWidth) {
			return cols
		}
	}
	return 1
}

func gridFits(entries []string, cols, termWidth int) bool {
	rows := (len(entries) + cols - 1) / cols
	total := 0
	for c := 0; c < cols; c++ {
		colWidth := columnWidth(entries, c, rows)
		if c < cols-1 {
			total += colWidth + columnGap
		} else {
			total += colWidth
		}
		if total > termWidth {
			return false
		}
	}
	return true
}

func columnWidth(entries []string, col, rows int) int {
	maxW := 0
	for row := 0; row < rows; row++ {
		idx := col*rows + row
		if idx >= len(entries) {
			break
		}
		w := utf8.RuneCountInString(entries[idx])
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

func buildGrid(entries []string, cols int) [][]string {
	rows := (len(entries) + cols - 1) / cols
	grid := make([][]string, rows)
	for r := 0; r < rows; r++ {
		row := make([]string, 0, cols)
		for c := 0; c < cols; c++ {
			idx := c*rows + r
			if idx >= len(entries) {
				break
			}
			row = append(row, entries[idx])
		}
		grid[r] = row
	}
	return grid
}
