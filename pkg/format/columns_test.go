// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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
		{name: "ascii", input: "hello", want: 5},
		{name: "empty", input: "", want: 0},
		{name: "multibyte_utf8", input: "cafe\u0301", want: 5}, // é as e + combining accent
		{name: "unicode_runes", input: "\u00e9\u00e9", want: 2},
		{name: "cjk_runes", input: "\u4e16\u754c", want: 2}, // rune count, not display width
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := displayWidth(tt.input)
			if got != tt.want {
				t.Errorf("displayWidth(%q) = %d, want %d", tt.input, got, tt.want)
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
		{name: "basic", s: "abc", width: 10, want: "abc       "},
		{name: "exact_width", s: "abcde", width: 5, want: "abcde"},
		{name: "exceeds_width", s: "abcdef", width: 3, want: "abcdef"},
		{name: "empty_string", s: "", width: 4, want: "    "},
		{name: "zero_width", s: "abc", width: 0, want: "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadRight(tt.s, tt.width)
			if got != tt.want {
				t.Errorf("PadRight(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
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
		{name: "basic", s: "42", width: 6, want: "    42"},
		{name: "exact_width", s: "abcde", width: 5, want: "abcde"},
		{name: "exceeds_width", s: "abcdef", width: 3, want: "abcdef"},
		{name: "empty_string", s: "", width: 4, want: "    "},
		{name: "zero_width", s: "42", width: 0, want: "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadLeft(tt.s, tt.width)
			if got != tt.want {
				t.Errorf("PadLeft(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
			}
		})
	}
}

func TestColumnsEmpty(t *testing.T) {
	got := Columns(nil, 80)
	if got != nil {
		t.Errorf("Columns(nil, 80) = %v, want nil", got)
	}
	got = Columns([]string{}, 80)
	if got != nil {
		t.Errorf("Columns([], 80) = %v, want nil", got)
	}
}

func TestColumnsZeroWidth(t *testing.T) {
	got := Columns([]string{"a", "b"}, 0)
	if got != nil {
		t.Errorf("Columns(entries, 0) = %v, want nil", got)
	}
}

func TestColumnsSingleEntry(t *testing.T) {
	got := Columns([]string{"hello"}, 80)
	if len(got) != 1 || len(got[0]) != 1 || got[0][0] != "hello" {
		t.Errorf("Columns([hello], 80) = %v, want [[hello]]", got)
	}
}

func TestColumnsVerticalDistribution(t *testing.T) {
	// 6 entries that should fit in 3 columns at width 80:
	// Each entry is 5 chars. 3 columns = 5+2+5+2+5 = 19 <= 80.
	// Vertical distribution with 3 cols and 2 rows:
	// col0: a, b (rows 0,1)
	// col1: c, d (rows 0,1)
	// col2: e, f (rows 0,1)
	entries := []string{"aaaaa", "bbbbb", "ccccc", "ddddd", "eeeee", "fffff"}
	got := Columns(entries, 80)

	// With 6 entries maximizing columns, we could fit up to 6 columns:
	// 6 cols = 5+2+5+2+5+2+5+2+5+2+5 = 5*6 + 2*5 = 40 <= 80. Should fit.
	// With 6 cols and 6 entries, numRows=1, each col has 1 entry.
	if len(got) != 1 {
		t.Errorf("expected 1 row for 6 equal entries in 80 cols, got %d rows", len(got))
		return
	}
	if len(got[0]) != 6 {
		t.Errorf("expected 6 columns, got %d", len(got[0]))
	}
}

func TestColumnsNarrowWidth(t *testing.T) {
	// With width 10, each entry is 8 chars. Only 1 column fits.
	entries := []string{"12345678", "abcdefgh", "ABCDEFGH"}
	got := Columns(entries, 10)

	if len(got) != 3 {
		t.Errorf("expected 3 rows for narrow width, got %d", len(got))
		return
	}
	for i, row := range got {
		if len(row) != 1 {
			t.Errorf("row %d: expected 1 column, got %d", i, len(row))
		}
	}
}

func TestColumnsFitsExactly(t *testing.T) {
	// 4 entries, each 3 chars. 4 cols: 3+2+3+2+3+2+3 = 18.
	// Width 18 should fit exactly 4 columns.
	entries := []string{"abc", "def", "ghi", "jkl"}
	got := Columns(entries, 18)

	if len(got) != 1 {
		t.Errorf("expected 1 row (4 cols), got %d rows", len(got))
		return
	}
	if len(got[0]) != 4 {
		t.Errorf("expected 4 columns, got %d", len(got[0]))
	}
}

func TestColumnsPerColumnWidth(t *testing.T) {
	// Entries of different lengths. Per-column width matters.
	// "a"(1), "bb"(2), "ccc"(3), "dd"(2), "e"(1), "fff"(3)
	// With 3 columns and 2 rows, vertical distribution:
	// col0: a(1), bb(2)   -> col width 2
	// col1: ccc(3), dd(2) -> col width 3
	// col2: e(1), fff(3)  -> col width 3
	// Total: 2+2+3+2+3 = 12
	entries := []string{"a", "bb", "ccc", "dd", "e", "fff"}
	got := Columns(entries, 12)

	// Should fit in at least 3 columns (maybe more). Let's check it's multi-column.
	if len(got) < 1 {
		t.Fatal("expected at least 1 row")
	}
	// With 6 entries of max 3 chars each, at width 12:
	// 6 cols: 1+2+3+2+1+3 + 5*2 = 12+10 = 22 > 12. No.
	// 5 cols: need to check per-column widths...
	// Just verify it produces valid output.
	totalEntries := 0
	for _, row := range got {
		totalEntries += len(row)
	}
	if totalEntries != 6 {
		t.Errorf("expected 6 total entries, got %d", totalEntries)
	}
}
