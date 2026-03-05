// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides shared output formatting for Unix utilities:
// column alignment (prd003-format R1), ANSI terminal colorization (R2),
// and human-readable size conversion (R3).
//
// Implements: prd003-format
// Architecture: docs/ARCHITECTURE.yaml § pkg/format
package format

import (
	"strings"
	"unicode/utf8"
)

// columnSep is the number of spaces inserted between adjacent columns,
// matching gls column layout.
const columnSep = 2

// displayWidth returns the number of runes in s, used as its display column
// width. East-Asian full-width characters are not distinguished; rune count
// is sufficient for ASCII-dominant utility output (prd003-format R1.3).
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// PadRight returns s right-padded with spaces to the given display width.
// If s already fills or exceeds width, it is returned unchanged.
// Implements prd003-format R1.2.
func PadRight(s string, width int) string {
	n := displayWidth(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

// PadLeft returns s left-padded with spaces to the given display width.
// If s already fills or exceeds width, it is returned unchanged.
// Implements prd003-format R1.2.
func PadLeft(s string, width int) string {
	n := displayWidth(s)
	if n >= width {
		return s
	}
	return strings.Repeat(" ", width-n) + s
}

// Columns distributes entries into the maximum number of columns that fit
// within termWidth terminal columns. Entries are arranged in row-major
// (left-to-right, top-to-bottom) order, matching gls --format=across
// behavior. Each column's width is the maximum display width of its entries.
// Returned cells are right-padded to their column width so the caller can
// join a row with a two-space separator and stay within termWidth.
// Implements prd003-format R1.1.
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}

	// Find the maximum column count whose total row width fits in termWidth.
	// Try descending from len(entries) to 1; stop at the first fit.
	bestCols := 1
	for numCols := len(entries); numCols >= 1; numCols-- {
		widths := colWidthsFor(entries, numCols)
		total := columnSep * (numCols - 1)
		for _, w := range widths {
			total += w
		}
		if total <= termWidth {
			bestCols = numCols
			break
		}
	}

	colWidths := colWidthsFor(entries, bestCols)
	numRows := (len(entries) + bestCols - 1) / bestCols

	grid := make([][]string, numRows)
	for r := range grid {
		grid[r] = make([]string, 0, bestCols)
	}
	for i, entry := range entries {
		r := i / bestCols
		c := i % bestCols
		grid[r] = append(grid[r], PadRight(entry, colWidths[c]))
	}
	return grid
}

// colWidthsFor computes the maximum display width per column for a row-major
// layout with numCols columns.
func colWidthsFor(entries []string, numCols int) []int {
	widths := make([]int, numCols)
	for i, entry := range entries {
		c := i % numCols
		if w := displayWidth(entry); w > widths[c] {
			widths[c] = w
		}
	}
	return widths
}
