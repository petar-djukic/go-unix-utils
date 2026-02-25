// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// column alignment, ANSI color output, and human-readable unit conversion.
//
// Implements: prd003-format (R1, R2, R3)
package format

import (
	"strings"
	"unicode/utf8"
)

// displayWidth returns the display width of a string measured in rune count.
// Multi-byte UTF-8 sequences count as single display positions.
//
// Per prd003-format R1.3: column width must be computed from display width
// (rune count), not byte count.
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// PadRight returns s padded with trailing spaces to the given display width.
// If the display width of s is already >= width, s is returned unchanged.
//
// Per prd003-format R1.2.
func PadRight(s string, width int) string {
	dw := displayWidth(s)
	if dw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-dw)
}

// PadLeft returns s padded with leading spaces to the given display width.
// If the display width of s is already >= width, s is returned unchanged.
//
// Per prd003-format R1.2.
func PadLeft(s string, width int) string {
	dw := displayWidth(s)
	if dw >= width {
		return s
	}
	return strings.Repeat(" ", width-dw) + s
}

// Columns distributes entries into the maximum number of columns that fit
// within termWidth. Column widths are computed from the longest entry in each
// column, not the global maximum. Entries are distributed vertically: down
// columns, then across.
//
// Returns a slice of rows, where each row is a slice of entry strings.
// Returns nil when entries is empty or termWidth is zero.
//
// Per prd003-format R1.1, R1.4.
// Utility context: ls default output places filenames in as many columns as
// fit in the terminal width (ls.c multi-column layout).
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 || termWidth <= 0 {
		return nil
	}

	// Try the maximum possible number of columns (len(entries)) down to 1.
	// For each candidate column count, compute the layout and check if it fits.
	bestCols := 1
	for numCols := len(entries); numCols >= 1; numCols-- {
		if fitsInWidth(entries, numCols, termWidth) {
			bestCols = numCols
			break
		}
	}

	return buildGrid(entries, bestCols)
}

// fitsInWidth checks whether entries distributed into numCols columns
// (vertically) fit within termWidth. Columns are separated by two spaces.
func fitsInWidth(entries []string, numCols, termWidth int) bool {
	numRows := (len(entries) + numCols - 1) / numCols
	totalWidth := 0

	for col := range numCols {
		colWidth := 0
		for row := range numRows {
			idx := col*numRows + row
			if idx >= len(entries) {
				continue
			}
			dw := displayWidth(entries[idx])
			if dw > colWidth {
				colWidth = dw
			}
		}
		totalWidth += colWidth
		// Add separator between columns (2 spaces), except after last column.
		if col < numCols-1 {
			totalWidth += 2
		}
	}

	return totalWidth <= termWidth
}

// buildGrid arranges entries into a grid with the given number of columns.
// Entries are distributed vertically (down columns, then across).
func buildGrid(entries []string, numCols int) [][]string {
	numRows := (len(entries) + numCols - 1) / numCols
	grid := make([][]string, numRows)

	for row := range numRows {
		var rowEntries []string
		for col := range numCols {
			idx := col*numRows + row
			if idx >= len(entries) {
				break
			}
			rowEntries = append(rowEntries, entries[idx])
		}
		grid[row] = rowEntries
	}

	return grid
}
