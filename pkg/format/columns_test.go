// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"strings"
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
		{"shorter", "ab", 5, "ab   "},
		{"exact", "abcde", 5, "abcde"},
		{"longer", "abcde", 3, "abcde"},
		{"empty to width", "", 3, "   "},
		{"zero width", "ab", 0, "ab"},
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
		{"shorter", "ab", 5, "   ab"},
		{"exact", "abcde", 5, "abcde"},
		{"longer", "abcde", 3, "abcde"},
		{"empty to width", "", 3, "   "},
		{"zero width", "ab", 0, "ab"},
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

func TestColumns_nil(t *testing.T) {
	t.Parallel()
	if result := Columns(nil, 80); result != nil {
		t.Errorf("Columns(nil, 80) = %v, want nil", result)
	}
}

func TestColumns_empty(t *testing.T) {
	t.Parallel()
	if result := Columns([]string{}, 80); len(result) != 0 {
		t.Errorf("Columns([], 80) = %v, want empty", result)
	}
}

func TestColumns_zeroTermWidth(t *testing.T) {
	t.Parallel()

	// termWidth == 0 → single column.
	result := Columns([]string{"hello"}, 0)
	if len(result) != 1 || len(result[0]) != 1 || result[0][0] != "hello" {
		t.Errorf("Columns([hello], 0) = %v, want [[hello]]", result)
	}

	// termWidth < 0 → single column.
	result2 := Columns([]string{"a", "b", "c"}, -1)
	if len(result2) != 3 {
		t.Errorf("Columns([a,b,c], -1) = %v, want 3 rows", result2)
	}
	for _, row := range result2 {
		if len(row) != 1 {
			t.Errorf("expected single-element row, got %v", row)
		}
	}
}

func TestColumns_fitsInWidth(t *testing.T) {
	t.Parallel()
	// "a"(1), "bb"(2), "ccc"(3), "d"(1) with termWidth=10.
	// 4 cols: widths [1,2,3,1], total = 1+2+3+1 + 3 separators = 10 → fits.
	result := Columns([]string{"a", "bb", "ccc", "d"}, 10)
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	// Each row's joined content must fit within 10.
	for _, row := range result {
		rowStr := strings.Join(row, " ")
		if len(rowStr) > 10 {
			t.Errorf("row %q has length %d, exceeds termWidth 10", rowStr, len(rowStr))
		}
	}
	// All four entries must appear (trimming padding).
	var got []string
	for _, row := range result {
		for _, cell := range row {
			got = append(got, strings.TrimRight(cell, " "))
		}
	}
	want := map[string]bool{"a": true, "bb": true, "ccc": true, "d": true}
	for _, g := range got {
		delete(want, g)
	}
	if len(want) != 0 {
		t.Errorf("missing entries %v in result %v", want, result)
	}
}

func TestColumns_entryWiderThanTermWidth(t *testing.T) {
	t.Parallel()
	// Entry wider than termWidth → single column, entry still present.
	result := Columns([]string{"verylongfilename"}, 5)
	if len(result) != 1 || len(result[0]) != 1 || result[0][0] != "verylongfilename" {
		t.Errorf("Columns([verylongfilename], 5) = %v, want [[verylongfilename]]", result)
	}
}

func TestColumns_multipleEntriesNoFit(t *testing.T) {
	t.Parallel()
	// Two long entries that cannot share a row → single column.
	result := Columns([]string{"longname", "anotherlongname"}, 5)
	if len(result) != 2 {
		t.Fatalf("expected 2 rows, got %d: %v", len(result), result)
	}
	for _, row := range result {
		if len(row) != 1 {
			t.Errorf("expected single-element row, got %v", row)
		}
	}
}

func TestColumns_columnMajorOrder(t *testing.T) {
	t.Parallel()
	// Entries [a,b,c,d], termWidth large enough for 2 cols but not 4.
	// nrows=2, col 0 = entries 0,1 = a,b; col 1 = entries 2,3 = c,d.
	// row 0 = [a, c], row 1 = [b, d].
	result := Columns([]string{"a", "b", "c", "d"}, 5)
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	// First column must be a and b (in rows 0 and 1).
	if len(result) >= 2 && len(result[0]) >= 1 && len(result[1]) >= 1 {
		col0row0 := strings.TrimRight(result[0][0], " ")
		col0row1 := strings.TrimRight(result[1][0], " ")
		if col0row0 != "a" || col0row1 != "b" {
			t.Errorf("expected first column [a,b], got [%s,%s]", col0row0, col0row1)
		}
	}
}
