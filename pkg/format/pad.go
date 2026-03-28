// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R1.1–R1.3: PadRight, PadLeft, and Columns
// for column-aligned tabular output.
package format

import (
	"strings"
	"unicode/utf8"
)

// columnGap is the minimum number of spaces between columns.
const columnGap = 2

// PadRight right-pads s with spaces to the given display width.
// Returns s unchanged if its rune count is already at or beyond width.
//
// R1.2: right-aligned field padding within a fixed-width column.
// R1.3: uses rune count for display width, not byte count.
func PadRight(s string, width int) string {
	runeLen := utf8.RuneCountInString(s)
	if runeLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-runeLen)
}

// PadLeft left-pads s with spaces to the given display width.
// Returns s unchanged if its rune count is already at or beyond width.
//
// R1.2: left-aligned field padding within a fixed-width column.
// R1.3: uses rune count for display width, not byte count.
func PadLeft(s string, width int) string {
	runeLen := utf8.RuneCountInString(s)
	if runeLen >= width {
		return s
	}
	return strings.Repeat(" ", width-runeLen) + s
}

// Columns arranges entries into the maximum number of columns that fit
// within termWidth, returning a slice of rows where each row is a slice
// of strings.
//
// R1.1: distributes entries into columns fitting termWidth, using the
// longest entry in each column to determine column width.
// R1.3: uses rune count for display width.
// R1.4: matches ls multi-column layout where column widths are per-column.
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}
	if termWidth <= 0 {
		return singleColumn(entries)
	}

	numCols := findMaxColumns(entries, termWidth)
	return layoutColumns(entries, numCols)
}

// findMaxColumns determines the maximum number of columns that fit.
func findMaxColumns(entries []string, termWidth int) int {
	n := len(entries)
	best := 1

	for cols := n; cols > 1; cols-- {
		if fitsInWidth(entries, cols, termWidth) {
			best = cols
			break
		}
	}
	return best
}

// fitsInWidth checks whether entries arranged in numCols columns fit
// within termWidth.
func fitsInWidth(entries []string, numCols, termWidth int) bool {
	rows := (len(entries) + numCols - 1) / numCols
	totalWidth := 0

	for col := range numCols {
		colMax := maxRuneWidthInColumn(entries, col, rows)
		if col < numCols-1 {
			totalWidth += colMax + columnGap
		} else {
			totalWidth += colMax
		}
		if totalWidth > termWidth {
			return false
		}
	}
	return true
}

// maxRuneWidthInColumn returns the maximum rune width of entries in a
// given column, where entries are laid out top-to-bottom then left-to-right.
func maxRuneWidthInColumn(entries []string, col, rows int) int {
	maxW := 0
	for row := range rows {
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

// layoutColumns arranges entries into rows with the given column count.
// Entries are distributed top-to-bottom, then left-to-right (down columns).
func layoutColumns(entries []string, numCols int) [][]string {
	rows := (len(entries) + numCols - 1) / numCols
	result := make([][]string, rows)

	for row := range rows {
		rowEntries := make([]string, 0, numCols)
		for col := range numCols {
			idx := col*rows + row
			if idx < len(entries) {
				rowEntries = append(rowEntries, entries[idx])
			}
		}
		result[row] = rowEntries
	}
	return result
}

// singleColumn returns all entries as single-element rows.
func singleColumn(entries []string) [][]string {
	result := make([][]string, len(entries))
	for i, e := range entries {
		result[i] = []string{e}
	}
	return result
}
