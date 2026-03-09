// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"testing"
)

func TestPadRight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{"shorter", "hi", 5, "hi   "},
		{"exact", "hello", 5, "hello"},
		{"longer", "longer", 3, "longer"},
		{"empty", "", 4, "    "},
		{"unicode", "café", 6, "café  "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PadRight(tc.s, tc.width)
			if got != tc.want {
				t.Errorf("PadRight(%q, %d) = %q, want %q", tc.s, tc.width, got, tc.want)
			}
		})
	}
}

func TestPadLeft(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{"shorter", "42", 5, "   42"},
		{"exact", "hello", 5, "hello"},
		{"longer", "longer", 3, "longer"},
		{"empty", "", 3, "   "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PadLeft(tc.s, tc.width)
			if got != tc.want {
				t.Errorf("PadLeft(%q, %d) = %q, want %q", tc.s, tc.width, got, tc.want)
			}
		})
	}
}

func TestColumnsBasic(t *testing.T) {
	t.Parallel()

	// AC5: column-first fill, respects termWidth.
	entries := []string{"a", "b", "c", "d", "e", "f"}
	rows := Columns(entries, 20)

	// With termWidth=20 and short entries, should fit multiple columns.
	if len(rows) == 0 {
		t.Fatal("Columns returned no rows")
	}
	// Verify column-first ordering: first column should contain the first entries.
	if rows[0][0] != "a" {
		t.Errorf("rows[0][0] = %q, want %q", rows[0][0], "a")
	}
}

func TestColumnsSingleLongEntry(t *testing.T) {
	t.Parallel()

	// AC5: single long entry falls back to one column.
	entries := []string{"this-is-a-very-long-filename-that-exceeds-width"}
	rows := Columns(entries, 20)

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if len(rows[0]) != 1 {
		t.Fatalf("expected 1 column, got %d", len(rows[0]))
	}
	if rows[0][0] != entries[0] {
		t.Errorf("got %q, want %q", rows[0][0], entries[0])
	}
}

func TestColumnsEmpty(t *testing.T) {
	t.Parallel()

	rows := Columns(nil, 80)
	if rows != nil {
		t.Errorf("Columns(nil, 80) = %v, want nil", rows)
	}
}

func TestColumnsColumnFirstOrder(t *testing.T) {
	t.Parallel()

	// 6 entries, termWidth=20, each entry is 3 chars.
	// With 3-char entries + 2-char gap = 5 per col, 20/5=4 cols max.
	// 4 cols → 2 rows. Column-first: col0=[a,b], col1=[c,d], col2=[e,f]
	// → 3 cols × 2 rows. Row0: a,c,e. Row1: b,d,f.
	entries := []string{"aaa", "bbb", "ccc", "ddd", "eee", "fff"}
	rows := Columns(entries, 20)

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	// Row 0 should be the first item from each column.
	wantRow0 := []string{"aaa", "ccc", "eee"}
	wantRow1 := []string{"bbb", "ddd", "fff"}

	for i, want := range wantRow0 {
		if i >= len(rows[0]) || rows[0][i] != want {
			t.Errorf("row0[%d] = %q, want %q", i, safeIndex(rows[0], i), want)
		}
	}
	for i, want := range wantRow1 {
		if i >= len(rows[1]) || rows[1][i] != want {
			t.Errorf("row1[%d] = %q, want %q", i, safeIndex(rows[1], i), want)
		}
	}
}

func TestColumnsNarrowWidth(t *testing.T) {
	t.Parallel()

	// termWidth too narrow for two columns → one column.
	entries := []string{"abcde", "fghij", "klmno"}
	rows := Columns(entries, 8)

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (single column), got %d", len(rows))
	}
	for i, e := range entries {
		if len(rows[i]) != 1 || rows[i][0] != e {
			t.Errorf("row[%d] = %v, want [%q]", i, rows[i], e)
		}
	}
}

func safeIndex(s []string, i int) string {
	if i >= len(s) {
		return "<missing>"
	}
	return s[i]
}
