// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R1.1-R1.4: column alignment and padding functions
// for multi-column tabular output matching GNU ls column-layout behavior.
//
// R3.5: ls -h shows file sizes in human-readable form in -l output; PadLeft
// right-aligns HumanSize output in fixed-width size columns. The same
// conversion applies to ls -s block counts.
// R3.6: du -h outputs directory sizes as human-readable strings; PadLeft
// right-aligns HumanSize output for consistent tabular du output.

package format

import "strings"

// PadRight pads s with trailing spaces to the given width. If s is already
// at or beyond width, it is returned unchanged.
//
// R1.2: left-aligned field padding within a fixed-width column.
func PadRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// PadLeft pads s with leading spaces to the given width. If s is already
// at or beyond width, it is returned unchanged.
//
// R1.2: right-aligned field padding within a fixed-width column.
func PadLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// columnGap is the number of spaces between columns, matching GNU ls.
const columnGap = 2

// Columns arranges entries into the maximum number of columns that fit within
// termWidth. Each column is sized to its widest entry, with two spaces between
// columns. Entries fill column-first (down each column, then across), matching
// GNU ls default column-filling order.
//
// R1.1: distributes entries into max columns fitting termWidth.
// R1.4: per-column widths from longest entry in each column, not global max.
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}

	// R3.4: single-column fallback for zero/negative termWidth.
	if termWidth <= 0 {
		return singleColumn(entries)
	}

	// Check if any entry exceeds termWidth; if so, single column.
	for _, e := range entries {
		if len(e) > termWidth {
			return singleColumn(entries)
		}
	}

	// Try increasing column counts from max possible down to 1.
	// Max possible columns: termWidth / (1 char + 2 gap) roughly.
	bestCols := 1
	for numCols := len(entries); numCols > 1; numCols-- {
		rows := (len(entries) + numCols - 1) / numCols
		colWidths := computeColumnWidths(entries, numCols, rows)
		totalWidth := totalLayoutWidth(colWidths)
		if totalWidth <= termWidth {
			bestCols = numCols
			break
		}
	}

	rows := (len(entries) + bestCols - 1) / bestCols
	return buildGrid(entries, bestCols, rows)
}

// computeColumnWidths returns the width of each column when entries are
// distributed into numCols columns with the given number of rows,
// filling column-first.
func computeColumnWidths(entries []string, numCols, rows int) []int {
	widths := make([]int, numCols)
	for col := range numCols {
		for row := range rows {
			idx := col*rows + row
			if idx >= len(entries) {
				break
			}
			if l := len(entries[idx]); l > widths[col] {
				widths[col] = l
			}
		}
	}
	return widths
}

// totalLayoutWidth returns the total display width for a set of column widths
// including two-space gaps between columns.
func totalLayoutWidth(colWidths []int) int {
	total := 0
	for i, w := range colWidths {
		total += w
		if i < len(colWidths)-1 {
			total += columnGap
		}
	}
	return total
}

// singleColumn returns entries arranged in a single-column layout.
func singleColumn(entries []string) [][]string {
	result := make([][]string, len(entries))
	for i, e := range entries {
		result[i] = []string{e}
	}
	return result
}

// buildGrid constructs the row-major grid from column-first filled entries.
func buildGrid(entries []string, numCols, rows int) [][]string {
	result := make([][]string, rows)
	for row := range rows {
		var cols []string
		for col := range numCols {
			idx := col*rows + row
			if idx >= len(entries) {
				break
			}
			cols = append(cols, entries[idx])
		}
		result[row] = cols
	}
	return result
}

// FormatSizeColumn converts a byte count to a human-readable string and
// right-aligns it within a fixed-width column. This composes HumanSize with
// PadLeft to produce the size field used in tabular output.
//
// R3.5: ls -h uses this to right-align file sizes in -l output. The same
// conversion applies to ls -s block counts, where the caller multiplies
// blocks by block size before passing to this function.
// R3.6: du -h uses this to right-align directory sizes in tabular output.
func FormatSizeColumn(bytes int64, width int, opts HumanSizeOpts) string {
	return PadLeft(HumanSize(bytes, opts), width)
}
