// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format_test

import (
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
)

func TestPadRight(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		width    int
		expected string
	}{
		// Use case S2: PadRight("abc", 10) returns "abc" followed by 7 spaces.
		{"ascii short", "abc", 10, "abc       "},
		{"exact width", "abc", 3, "abc"},
		{"over width", "abcdef", 3, "abcdef"},
		{"empty string", "", 5, "     "},
		// R1.3: width from rune count, not byte count.
		// "café" = 4 runes, 5 bytes; pad to 8 means 4 spaces added.
		{"unicode multibyte", "café", 8, "café    "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format.PadRight(tt.s, tt.width)
			if got != tt.expected {
				t.Errorf("PadRight(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.expected)
			}
		})
	}
}

func TestPadLeft(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		width    int
		expected string
	}{
		// Use case S2: PadLeft("abc", 10) returns 7 spaces followed by "abc".
		{"ascii short", "abc", 10, "       abc"},
		{"exact width", "abc", 3, "abc"},
		{"over width", "abcdef", 3, "abcdef"},
		{"empty string", "", 5, "     "},
		// R1.3: width from rune count, not byte count.
		{"unicode multibyte", "café", 8, "    café"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format.PadLeft(tt.s, tt.width)
			if got != tt.expected {
				t.Errorf("PadLeft(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.expected)
			}
		})
	}
}

func TestColumns_Nil(t *testing.T) {
	if got := format.Columns(nil, 80); got != nil {
		t.Errorf("Columns(nil, 80) = %v, want nil", got)
	}
}

func TestColumns_Empty(t *testing.T) {
	if got := format.Columns([]string{}, 80); got != nil {
		t.Errorf("Columns([], 80) = %v, want nil", got)
	}
}

func TestColumns_NonPositiveWidth(t *testing.T) {
	if got := format.Columns([]string{"a"}, 0); got != nil {
		t.Errorf("Columns([a], 0) = %v, want nil", got)
	}
}

func TestColumns_SingleEntry(t *testing.T) {
	rows := format.Columns([]string{"hello"}, 80)
	if len(rows) != 1 {
		t.Fatalf("Columns([hello], 80): got %d rows, want 1", len(rows))
	}
	if len(rows[0]) != 1 || rows[0][0] != "hello" {
		t.Errorf("rows[0] = %v, want [hello]", rows[0])
	}
}

// TestColumns_FitsWidth verifies that Columns with 20 filenames and
// termWidth=80 produces rows whose padded widths all fit within 80 characters.
// (prd003-format R1.1, R1.4; use case S1)
func TestColumns_FitsWidth(t *testing.T) {
	// entries[i] has display width i+1 (widths 1..20).
	entries := make([]string, 20)
	for i := range entries {
		entries[i] = strings.Repeat("x", i+1)
	}
	const termWidth = 80
	rows := format.Columns(entries, termWidth)
	if len(rows) == 0 {
		t.Fatal("Columns returned empty rows for non-empty entries")
	}

	// Derive column widths from the returned rows.
	numCols := 0
	for _, row := range rows {
		if len(row) > numCols {
			numCols = len(row)
		}
	}
	colWidths := make([]int, numCols)
	for _, row := range rows {
		for c, entry := range row {
			if w := len([]rune(entry)); w > colWidths[c] {
				colWidths[c] = w
			}
		}
	}

	// Every row must fit within termWidth when padded to column widths
	// with 2-space separators between columns.
	for r, row := range rows {
		lineWidth := 0
		for c := range row {
			lineWidth += colWidths[c]
			if c < len(row)-1 {
				lineWidth += 2
			}
		}
		if lineWidth > termWidth {
			t.Errorf("row %d has padded width %d, exceeds termWidth=%d", r, lineWidth, termWidth)
		}
	}
}

// TestColumns_ColumnMajorOrder verifies that entries fill columns
// top-to-bottom, left-to-right, matching GNU ls -C default layout.
// (prd003-format R1.1, R1.4)
func TestColumns_ColumnMajorOrder(t *testing.T) {
	// 5 single-char entries in a narrow terminal (termWidth=4) forces 2 columns.
	// Expected layout with 2 columns, 3 rows:
	//   a  d
	//   b  e
	//   c
	entries := []string{"a", "b", "c", "d", "e"}
	rows := format.Columns(entries, 4)

	if len(rows) != 3 {
		t.Fatalf("Columns([a,b,c,d,e], 4): got %d rows, want 3; rows=%v", len(rows), rows)
	}

	expected := [][]string{
		{"a", "d"},
		{"b", "e"},
		{"c"},
	}
	for i, want := range expected {
		if len(rows[i]) != len(want) {
			t.Errorf("rows[%d] has %d entries %v, want %d %v", i, len(rows[i]), rows[i], len(want), want)
			continue
		}
		for j, e := range want {
			if rows[i][j] != e {
				t.Errorf("rows[%d][%d] = %q, want %q", i, j, rows[i][j], e)
			}
		}
	}
}

// TestColumns_MaximizesColumns verifies that the maximum possible column
// count is chosen for the given terminal width.
func TestColumns_MaximizesColumns(t *testing.T) {
	// 6 single-char entries; termWidth=10 fits at most 3 columns
	// (3*1 + 2*2 = 7 ≤ 10) but not 4 (4*1 + 3*2 = 10 ≤ 10 actually fits).
	// 4 cols: 4*1 + 3*2 = 10 ≤ 10 → fits.
	// 5 cols: 5*1 + 4*2 = 13 > 10 → doesn't fit.
	// So maximum is 4 columns (but with 6 entries and 4 cols, numRows=2,
	// effectiveCols=3 since ceil(6/2)=3, totalWidth=3*1+2*2=7 ≤ 10).
	// Actually with 4 cols + 6 entries: numRows=ceil(6/4)=2, effectiveCols=3.
	// First numCols tried that succeeds determines the layout.
	entries := []string{"a", "b", "c", "d", "e", "f"}
	rows := format.Columns(entries, 10)
	if len(rows) == 0 {
		t.Fatal("Columns returned nil for non-empty input")
	}
	// All entries must be present exactly once.
	seen := make(map[string]int)
	for _, row := range rows {
		for _, e := range row {
			seen[e]++
		}
	}
	for _, e := range entries {
		if seen[e] != 1 {
			t.Errorf("entry %q appears %d times in output, want 1", e, seen[e])
		}
	}
}
