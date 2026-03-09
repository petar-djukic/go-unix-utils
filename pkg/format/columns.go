// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd003-format R3.1–R3.4 (PadRight, PadLeft, Columns, visibleWidth).
package format

import (
	"strings"
	"unicode/utf8"
)

// columnSeparatorWidth is the number of spaces between columns in Columns output.
const columnSeparatorWidth = 2

// visibleWidth returns the display width of s by stripping ANSI escape sequences
// and counting runes. R3.4.
func visibleWidth(s string) int {
	return utf8.RuneCountInString(stripANSI(s))
}

// stripANSI removes ANSI escape sequences (CSI sequences: ESC[ ... final byte)
// from s. R3.4, D2.
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Skip ESC[, parameter bytes (0x30-0x3F), intermediate bytes (0x20-0x2F),
			// and the final byte (0x40-0x7E).
			j := i + 2
			for j < len(s) && s[j] >= 0x30 && s[j] <= 0x3F {
				j++
			}
			for j < len(s) && s[j] >= 0x20 && s[j] <= 0x2F {
				j++
			}
			if j < len(s) && s[j] >= 0x40 && s[j] <= 0x7E {
				j++
			}
			i = j
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// PadRight right-pads s with spaces to the given width based on visible width
// (excluding ANSI escape bytes). If the visible width is already at or beyond
// width, returns s unchanged. R3.1, D4.
func PadRight(s string, width int) string {
	vw := visibleWidth(s)
	if vw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vw)
}

// PadLeft left-pads s with spaces to the given width based on visible width
// (excluding ANSI escape bytes). If the visible width is already at or beyond
// width, returns s unchanged. R3.2, D4.
func PadLeft(s string, width int) string {
	vw := visibleWidth(s)
	if vw >= width {
		return s
	}
	return strings.Repeat(" ", width-vw) + s
}

// Columns arranges entries into columns that fit within termWidth, filling
// top-to-bottom, left-to-right (column-major order, matching GNU ls). Returns
// a slice of rows, each row a slice of column entries. R3.3, D1, D3.
func Columns(entries []string, termWidth int) [][]string {
	if len(entries) == 0 {
		return nil
	}

	// Precompute visible widths.
	widths := make([]int, len(entries))
	for i, e := range entries {
		widths[i] = visibleWidth(e)
	}

	n := len(entries)

	// Try column counts from high to low. D3.
	for numCols := n; numCols > 1; numCols-- {
		numRows := (n + numCols - 1) / numCols

		// Compute per-column max width.
		colWidths := make([]int, numCols)
		for col := range numCols {
			for row := range numRows {
				idx := col*numRows + row
				if idx >= n {
					continue
				}
				if widths[idx] > colWidths[col] {
					colWidths[col] = widths[idx]
				}
			}
		}

		// Total width: sum of column widths + separators between columns.
		total := 0
		for i, cw := range colWidths {
			total += cw
			if i < numCols-1 {
				total += columnSeparatorWidth
			}
		}

		if total > termWidth {
			continue
		}

		// Build the rows.
		rows := make([][]string, numRows)
		for row := range numRows {
			var rowEntries []string
			for col := range numCols {
				idx := col*numRows + row
				if idx >= n {
					break
				}
				rowEntries = append(rowEntries, entries[idx])
			}
			rows[row] = rowEntries
		}
		return rows
	}

	// Single-column fallback. D3.
	rows := make([][]string, n)
	for i, e := range entries {
		rows[i] = []string{e}
	}
	return rows
}
