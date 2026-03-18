// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd003-format R1.1-R1.4: column alignment functions.
package format

import (
	"testing"
)

func TestPadRight(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"pad shorter", "abc", 6, "abc   "},
		{"no pad when equal", "abcdef", 6, "abcdef"},
		{"no pad when longer", "abcdef", 4, "abcdef"},
		{"empty string", "", 3, "   "},
		{"zero width", "abc", 0, "abc"},
		{"utf8 runes", "café", 6, "café  "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PadRight(tc.input, tc.width)
			if got != tc.want {
				t.Fatalf("PadRight(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
			}
		})
	}
}

func TestPadLeft(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"pad shorter", "42", 6, "    42"},
		{"no pad when equal", "abcdef", 6, "abcdef"},
		{"no pad when longer", "abcdef", 4, "abcdef"},
		{"empty string", "", 3, "   "},
		{"zero width", "abc", 0, "abc"},
		{"utf8 runes", "café", 6, "  café"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PadLeft(tc.input, tc.width)
			if got != tc.want {
				t.Fatalf("PadLeft(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
			}
		})
	}
}

func TestColumnsEmpty(t *testing.T) {
	t.Parallel()
	got := Columns(nil, 80)
	if got != nil {
		t.Fatalf("Columns(nil, 80) = %v, want nil", got)
	}
	got = Columns([]string{}, 80)
	if got != nil {
		t.Fatalf("Columns([], 80) = %v, want nil", got)
	}
}

func TestColumnsZeroWidth(t *testing.T) {
	t.Parallel()
	got := Columns([]string{"a", "b"}, 0)
	if got != nil {
		t.Fatalf("Columns with termWidth=0 = %v, want nil", got)
	}
	got = Columns([]string{"a", "b"}, -1)
	if got != nil {
		t.Fatalf("Columns with termWidth=-1 = %v, want nil", got)
	}
}

func TestColumnsSingleEntry(t *testing.T) {
	t.Parallel()
	got := Columns([]string{"hello"}, 80)
	if len(got) != 1 || len(got[0]) != 1 || got[0][0] != "hello" {
		t.Fatalf("Columns single entry = %v, want [[hello]]", got)
	}
}

func TestColumnsColumnFirstFill(t *testing.T) {
	t.Parallel()
	// 6 entries, width=80, each entry 1 char -> fits in many columns
	// With 3 columns and 6 entries: 2 rows
	// Column-first: col0=[a,b], col1=[c,d], col2=[e,f]
	// Row 0: a, c, e
	// Row 1: b, d, f
	entries := []string{"a", "b", "c", "d", "e", "f"}
	got := Columns(entries, 80)

	// With 1-char entries and 2-space gaps, 6 columns need 6*1 + 5*2 = 16 chars
	// which fits in 80. So all 6 in one row.
	if len(got) != 1 {
		// If 6 columns fit, we get 1 row with all entries
		if len(got[0]) != 6 {
			t.Fatalf("expected 6 entries in row, got %d", len(got[0]))
		}
		// Column-first with 1 row: each entry is its own column
		for i, want := range entries {
			if got[0][i] != want {
				t.Fatalf("row[0][%d] = %q, want %q", i, got[0][i], want)
			}
		}
		return
	}
}

func TestColumnsFallbackToSingleColumn(t *testing.T) {
	t.Parallel()
	// Entries wider than termWidth -> single column
	entries := []string{"longentry1", "longentry2", "longentry3"}
	got := Columns(entries, 5)
	if len(got) != 3 {
		t.Fatalf("expected 3 rows (single column), got %d", len(got))
	}
	for i, row := range got {
		if len(row) != 1 || row[0] != entries[i] {
			t.Fatalf("row %d = %v, want [%s]", i, row, entries[i])
		}
	}
}

func TestColumnsLayout(t *testing.T) {
	t.Parallel()
	// 5 entries of width 3, termWidth=15
	// 2-space gap: 2 cols need max 3+2+3=8, fits
	// 3 cols need 3+2+3+2+3=13, fits
	// 4 cols: rows=2, col0=[a,b](2 entries), col1=[c,d](2), col2=[e](1), col3=empty
	//   wait, 4 cols with 5 entries: rows=ceil(5/4)=2
	//   col0: idx 0,1 -> aaa,bbb; col1: idx 2,3 -> ccc,ddd; col2: idx 4 -> eee; col3: empty
	//   width = 3+2+3+2+3 = 13, fits in 15
	//   but only 3 cols have entries so effectively 3 columns
	// 5 cols: rows=1, all in one row: 3+2+3+2+3+2+3=18 > 15, doesn't fit
	// So max cols = 4 (but col3 is empty), giving rows=2
	entries := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	got := Columns(entries, 15)

	// With 4 cols, 5 entries, rows=2:
	// col0: [aaa, bbb], col1: [ccc, ddd], col2: [eee], col3: []
	// row0: aaa, ccc, eee
	// row1: bbb, ddd
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d: %v", len(got), got)
	}
	wantRow0 := []string{"aaa", "ccc", "eee"}
	wantRow1 := []string{"bbb", "ddd"}
	checkRow(t, got[0], wantRow0, 0)
	checkRow(t, got[1], wantRow1, 1)
}

// checkRow verifies a row matches expected entries.
func checkRow(t *testing.T, got, want []string, rowIdx int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("row %d: len=%d, want %d; got=%v", rowIdx, len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d col %d: %q, want %q", rowIdx, i, got[i], want[i])
		}
	}
}
