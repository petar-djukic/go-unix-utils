// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for Columns, PadRight, and PadLeft (prd003-format R1.1, R1.2, R1.3, R1.4).
package format

import (
	"testing"
)

func TestColumns(t *testing.T) {
	tests := []struct {
		name      string
		entries   []string
		termWidth int
		want      [][]string
	}{
		{
			name:      "empty input returns nil",
			entries:   []string{},
			termWidth: 80,
			want:      nil,
		},
		{
			name:      "single entry",
			entries:   []string{"hello"},
			termWidth: 80,
			want:      [][]string{{"hello"}},
		},
		{
			name:    "all entries fit in one row",
			entries: []string{"a", "bb", "ccc"},
			// 3 cols, 1 row: colWidths=[1,2,3], total=1+2+3+2*2=10 <= 80
			termWidth: 80,
			want:      [][]string{{"a", "bb", "ccc"}},
		},
		{
			name:    "entries wider than terminal force single column",
			entries: []string{"abcdef", "ghijkl", "mnopqr"},
			// Each entry is 6 chars, wider than termWidth=5.
			// Even 2 cols: 6+6+2=14 > 5. Falls to 1 col.
			termWidth: 5,
			want:      [][]string{{"abcdef"}, {"ghijkl"}, {"mnopqr"}},
		},
		{
			name:    "column-major ordering two columns",
			entries: []string{"a", "bb", "ccc", "d", "ee"},
			// numCols=2, numRows=3
			// col 0: a(1), bb(2), ccc(3) -> width=3
			// col 1: d(1), ee(2) -> width=2
			// total = 3+2+2 = 7 <= 10
			// Row 0: entries[0]="a", entries[3]="d"
			// Row 1: entries[1]="bb", entries[4]="ee"
			// Row 2: entries[2]="ccc"
			termWidth: 10,
			want: [][]string{
				{"a", "d"},
				{"bb", "ee"},
				{"ccc"},
			},
		},
		{
			name:    "column-major ordering three columns",
			entries: []string{"aa", "bb", "cc", "dd", "ee", "ff"},
			// numCols=3, numRows=2: colWidths=[2,2,2], total=6+2*2=10 <= 10
			// Row 0: entries[0]="aa", entries[2]="cc", entries[4]="ee"
			// Row 1: entries[1]="bb", entries[3]="dd", entries[5]="ff"
			termWidth: 10,
			want: [][]string{
				{"aa", "cc", "ee"},
				{"bb", "dd", "ff"},
			},
		},
		{
			name:    "uneven last column",
			entries: []string{"alpha", "beta", "gamma", "delta"},
			// 4 entries all fit in 1 row: widths=[5,4,5,5], total=19+2*3=25 <= 80
			termWidth: 80,
			want:      [][]string{{"alpha", "beta", "gamma", "delta"}},
		},
		{
			name:    "exact fit at terminal width",
			entries: []string{"abc", "def"},
			// 2 cols, 1 row: total = 3+3+2 = 8 <= 8
			termWidth: 8,
			want:      [][]string{{"abc", "def"}},
		},
		{
			name:    "one char too narrow forces fewer columns",
			entries: []string{"abc", "def"},
			// 2 cols, 1 row: total = 3+3+2 = 8 > 7
			// 1 col, 2 rows: total = 3 <= 7
			termWidth: 7,
			want:      [][]string{{"abc"}, {"def"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Columns(tt.entries, tt.termWidth)
			if tt.want == nil {
				if got != nil {
					t.Errorf("Columns() = %v; want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Columns() returned %d rows; want %d", len(got), len(tt.want))
			}
			for r, wantRow := range tt.want {
				gotRow := got[r]
				if len(gotRow) != len(wantRow) {
					t.Errorf("row %d: got %d entries %v; want %d entries %v", r, len(gotRow), gotRow, len(wantRow), wantRow)
					continue
				}
				for c, wantVal := range wantRow {
					if gotRow[c] != wantVal {
						t.Errorf("row %d col %d: got %q; want %q", r, c, gotRow[c], wantVal)
					}
				}
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{
			name:  "ASCII shorter than width",
			s:     "abc",
			width: 6,
			want:  "abc   ",
		},
		{
			name:  "ASCII equal to width",
			s:     "abc",
			width: 3,
			want:  "abc",
		},
		{
			name:  "ASCII exceeds width",
			s:     "abcdef",
			width: 3,
			want:  "abcdef",
		},
		{
			name:  "empty string",
			s:     "",
			width: 4,
			want:  "    ",
		},
		{
			name:  "multi-byte UTF-8 shorter than width",
			s:     "caf\u00e9",
			width: 8,
			want:  "caf\u00e9    ",
		},
		{
			name:  "multi-byte UTF-8 rune count equals width",
			s:     "caf\u00e9",
			width: 4,
			want:  "caf\u00e9",
		},
		{
			name:  "multi-byte UTF-8 rune count exceeds width",
			s:     "\u65e5\u672c\u8a9e",
			width: 2,
			want:  "\u65e5\u672c\u8a9e",
		},
		{
			name:  "width zero",
			s:     "abc",
			width: 0,
			want:  "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadRight(tt.s, tt.width)
			if got != tt.want {
				t.Errorf("PadRight(%q, %d) = %q; want %q", tt.s, tt.width, got, tt.want)
			}
		})
	}
}

func TestPadLeft(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{
			name:  "ASCII shorter than width",
			s:     "abc",
			width: 6,
			want:  "   abc",
		},
		{
			name:  "ASCII equal to width",
			s:     "abc",
			width: 3,
			want:  "abc",
		},
		{
			name:  "ASCII exceeds width",
			s:     "abcdef",
			width: 3,
			want:  "abcdef",
		},
		{
			name:  "empty string",
			s:     "",
			width: 4,
			want:  "    ",
		},
		{
			name:  "multi-byte UTF-8 shorter than width",
			s:     "caf\u00e9",
			width: 8,
			want:  "    caf\u00e9",
		},
		{
			name:  "multi-byte UTF-8 rune count equals width",
			s:     "caf\u00e9",
			width: 4,
			want:  "caf\u00e9",
		},
		{
			name:  "multi-byte UTF-8 rune count exceeds width",
			s:     "\u65e5\u672c\u8a9e",
			width: 2,
			want:  "\u65e5\u672c\u8a9e",
		},
		{
			name:  "width zero",
			s:     "abc",
			width: 0,
			want:  "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadLeft(tt.s, tt.width)
			if got != tt.want {
				t.Errorf("PadLeft(%q, %d) = %q; want %q", tt.s, tt.width, got, tt.want)
			}
		})
	}
}
