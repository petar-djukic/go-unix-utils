// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for align.go: PadRight, PadLeft, Columns, displayWidth, columnWidths, totalWidth.
// Implements: prd003-format (R1)
package format

import (
	"testing"
)

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "empty string",
			input: "",
			want:  0,
		},
		{
			name:  "ASCII string",
			input: "hello",
			want:  5,
		},
		{
			name:  "single ASCII character",
			input: "x",
			want:  1,
		},
		{
			name:  "multi-byte UTF-8 two-byte runes",
			input: "café",
			want:  4,
		},
		{
			name:  "multi-byte UTF-8 three-byte runes",
			input: "日本語",
			want:  3,
		},
		{
			name:  "mixed ASCII and multi-byte",
			input: "aé日",
			want:  3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := displayWidth(tc.input)
			if got != tc.want {
				t.Fatalf("displayWidth(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{
			name:  "shorter than width",
			input: "hi",
			width: 5,
			want:  "hi   ",
		},
		{
			name:  "equal to width",
			input: "hello",
			width: 5,
			want:  "hello",
		},
		{
			name:  "exceeding width",
			input: "hello world",
			width: 5,
			want:  "hello world",
		},
		{
			name:  "empty string padded",
			input: "",
			width: 3,
			want:  "   ",
		},
		{
			name:  "zero width",
			input: "abc",
			width: 0,
			want:  "abc",
		},
		{
			name:  "multi-byte UTF-8 shorter than width",
			input: "café",
			width: 6,
			want:  "café  ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PadRight(tc.input, tc.width)
			if got != tc.want {
				t.Fatalf("PadRight(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
			}
		})
	}
}

func TestPadLeft(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{
			name:  "shorter than width",
			input: "42",
			width: 5,
			want:  "   42",
		},
		{
			name:  "equal to width",
			input: "hello",
			width: 5,
			want:  "hello",
		},
		{
			name:  "exceeding width",
			input: "hello world",
			width: 5,
			want:  "hello world",
		},
		{
			name:  "empty string padded",
			input: "",
			width: 3,
			want:  "   ",
		},
		{
			name:  "zero width",
			input: "abc",
			width: 0,
			want:  "abc",
		},
		{
			name:  "multi-byte UTF-8 shorter than width",
			input: "café",
			width: 6,
			want:  "  café",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PadLeft(tc.input, tc.width)
			if got != tc.want {
				t.Fatalf("PadLeft(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
			}
		})
	}
}

func TestColumns(t *testing.T) {
	tests := []struct {
		name      string
		entries   []string
		termWidth int
		wantRows  int
		wantCols  int // expected number of columns in the first row
	}{
		{
			name:      "empty input returns nil",
			entries:   []string{},
			termWidth: 80,
			wantRows:  0,
			wantCols:  0,
		},
		{
			name:      "single entry",
			entries:   []string{"file.txt"},
			termWidth: 80,
			wantRows:  1,
			wantCols:  1,
		},
		{
			name:      "two entries fit in one row",
			entries:   []string{"aa", "bb"},
			termWidth: 80,
			wantRows:  1,
			wantCols:  2,
		},
		{
			name:      "entries too wide for multiple columns",
			entries:   []string{"a-very-long-filename.txt", "another-long-filename.txt"},
			termWidth: 30,
			wantRows:  2,
			wantCols:  1,
		},
		{
			name:      "three entries in narrow terminal",
			entries:   []string{"aaaa", "bbbb", "cccc"},
			termWidth: 6, // each entry is 4 wide, 4+2+4=10 > 6, so 1 column
			wantRows:  3,
			wantCols:  1,
		},
		{
			name:      "multiple entries across layout",
			entries:   []string{"a", "b", "c", "d", "e", "f"},
			termWidth: 80,
			wantRows:  1,
			wantCols:  6,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Columns(tc.entries, tc.termWidth)
			if tc.wantRows == 0 {
				if got != nil {
					t.Fatalf("Columns(%v, %d) = %v, want nil", tc.entries, tc.termWidth, got)
				}
				return
			}
			if len(got) != tc.wantRows {
				t.Fatalf("Columns(%v, %d) row count = %d, want %d", tc.entries, tc.termWidth, len(got), tc.wantRows)
			}
			if len(got[0]) != tc.wantCols {
				t.Fatalf("Columns(%v, %d) first row col count = %d, want %d", tc.entries, tc.termWidth, len(got[0]), tc.wantCols)
			}
		})
	}
}

func TestColumnsAcrossLayout(t *testing.T) {
	// Verify entries are distributed row-by-row (across layout).
	// With 4 entries and 2 columns: row 0 = [a, b], row 1 = [c, d].
	entries := []string{"a", "b", "c", "d"}
	got := Columns(entries, 6) // 1+2+1=4 fits in 6
	if got == nil {
		t.Fatal("Columns returned nil for non-empty input")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	// Row 0: a, b (across layout)
	if got[0][0] != "a" || got[0][1] != "b" {
		t.Fatalf("row 0: got %v, want [a, b]", got[0])
	}
	// Row 1: c, d
	if got[1][0] != "c" || got[1][1] != "d" {
		t.Fatalf("row 1: got %v, want [c, d]", got[1])
	}
}

func TestColumnsLastRowIncomplete(t *testing.T) {
	// 5 entries, 3 columns => row 0 = [a,b,c], row 1 = [d,e]
	entries := []string{"a", "b", "c", "d", "e"}
	got := Columns(entries, 80)
	if got == nil {
		t.Fatal("Columns returned nil for non-empty input")
	}
	// Last row should have fewer entries
	lastRow := got[len(got)-1]
	totalEntries := 0
	for _, row := range got {
		totalEntries += len(row)
	}
	if totalEntries != 5 {
		t.Fatalf("total entries across rows = %d, want 5", totalEntries)
	}
	// Verify the last row has the remainder
	if len(lastRow) > len(got[0]) {
		t.Fatalf("last row has more entries (%d) than first row (%d)", len(lastRow), len(got[0]))
	}
}

func TestColumnWidths(t *testing.T) {
	tests := []struct {
		name   string
		widths []int
		nCols  int
		want   []int
	}{
		{
			name:   "single column",
			widths: []int{3, 5, 2},
			nCols:  1,
			want:   []int{5},
		},
		{
			name:   "two columns across layout",
			widths: []int{3, 5, 2, 4},
			nCols:  2,
			// col 0: indices 0,2 -> max(3,2)=3; col 1: indices 1,3 -> max(5,4)=5
			want: []int{3, 5},
		},
		{
			name:   "three columns with uneven distribution",
			widths: []int{1, 2, 3, 4, 5},
			nCols:  3,
			// col 0: indices 0,3 -> max(1,4)=4; col 1: indices 1,4 -> max(2,5)=5; col 2: index 2 -> 3
			want: []int{4, 5, 3},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := columnWidths(tc.widths, tc.nCols)
			if len(got) != len(tc.want) {
				t.Fatalf("columnWidths(%v, %d) length = %d, want %d", tc.widths, tc.nCols, len(got), len(tc.want))
			}
			for i, w := range got {
				if w != tc.want[i] {
					t.Fatalf("columnWidths(%v, %d)[%d] = %d, want %d", tc.widths, tc.nCols, i, w, tc.want[i])
				}
			}
		})
	}
}

func TestTotalWidth(t *testing.T) {
	tests := []struct {
		name      string
		colWidths []int
		want      int
	}{
		{
			name:      "single column no separator",
			colWidths: []int{10},
			want:      10,
		},
		{
			name:      "two columns with separator",
			colWidths: []int{5, 8},
			want:      15, // 5 + 2 + 8
		},
		{
			name:      "three columns with separators",
			colWidths: []int{4, 5, 3},
			want:      16, // 4 + 2 + 5 + 2 + 3
		},
		{
			name:      "empty column list",
			colWidths: []int{},
			want:      0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := totalWidth(tc.colWidths)
			if got != tc.want {
				t.Fatalf("totalWidth(%v) = %d, want %d", tc.colWidths, got, tc.want)
			}
		})
	}
}
