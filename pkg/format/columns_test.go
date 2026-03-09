// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"reflect"
	"testing"
)

func TestColumnsEmpty(t *testing.T) {
	t.Parallel()
	got := Columns(nil, 80)
	if got != nil {
		t.Errorf("Columns(nil, 80) = %v, want nil", got)
	}
	got = Columns([]string{}, 80)
	if got != nil {
		t.Errorf("Columns([], 80) = %v, want nil", got)
	}
}

func TestColumnsSingleEntry(t *testing.T) {
	t.Parallel()
	got := Columns([]string{"hello"}, 80)
	want := [][]string{{"hello"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns([hello], 80) = %v, want %v", got, want)
	}
}

func TestColumnsOversizedEntry(t *testing.T) {
	t.Parallel()
	// Entries wider than termWidth produce single-column output.
	entries := []string{"abcdefghij", "klmnopqrst"}
	got := Columns(entries, 5)
	want := [][]string{{"abcdefghij"}, {"klmnopqrst"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns with oversized entries = %v, want %v", got, want)
	}
}

func TestColumnsColumnMajorOrder(t *testing.T) {
	t.Parallel()
	// 6 entries in 3 columns, column-major order:
	// col0: a, b  col1: c, d  col2: e, f
	// row0: a c e
	// row1: b d f
	entries := []string{"a", "b", "c", "d", "e", "f"}
	got := Columns(entries, 80)

	// With width 80 and single-char entries, all 6 should fit in 6 columns.
	// But let's verify column-major order holds regardless of column count.
	// Flatten column-major: for each col, top-to-bottom entries should be
	// consecutive from the original slice.
	// With 6 entries in 6 cols: each col has 1 row → [[a,b,c,d,e,f]]
	// With 6 entries in 3 cols: 2 rows → [[a,c,e],[b,d,f]]
	// With 6 entries in 2 cols: 3 rows → [[a,d],[b,e],[c,f]]
	// The function maximizes columns. Width 80, entries are 1 char each.
	// 6 cols: 6*1 + 5*2 = 16 ≤ 80 → fits.
	want := [][]string{{"a", "b", "c", "d", "e", "f"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns 6 entries width 80 = %v, want %v", got, want)
	}
}

func TestColumnsColumnMajorNarrow(t *testing.T) {
	t.Parallel()
	// Force 3 columns: entries "aa" (2 chars each), 6 entries.
	// 3 cols: 2 rows, widths 2+2+2+2*2 = 10 → need termWidth >= 10.
	// 6 cols: 6*2 + 5*2 = 22.
	// 4 cols: 2 rows. Col widths: col0=[0,1]→2, col1=[2,3]→2, col2=[4,5]→2, col3=[6,7]→overflow at idx 6,7 with n=6.
	// Actually with n=6, 4 cols: rows=2. col0: idx 0,1. col1: idx 2,3. col2: idx 4,5. col3: idx 6,7 → 6,7 out of range → col3 is empty.
	// That's effectively 3 cols. So 4 cols would produce: rows=2, [[a1,a3,a5],[a2,a4,a6]] — same as 3 cols.
	// Let me use termWidth=10 to force exactly 3 columns.
	entries := []string{"a1", "a2", "a3", "a4", "a5", "a6"}
	got := Columns(entries, 10)
	// 3 cols: rows=2. col0: [0,1]="a1","a2". col1: [2,3]="a3","a4". col2: [4,5]="a5","a6".
	// row0: a1, a3, a5.  row1: a2, a4, a6.
	want := [][]string{{"a1", "a3", "a5"}, {"a2", "a4", "a6"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns 6 entries width 10 = %v, want %v", got, want)
	}
}

func TestColumnsUnevenDistribution(t *testing.T) {
	t.Parallel()
	// 5 entries forced into 2 columns: rows=3.
	// col0: idx 0,1,2 → e1,e2,e3. col1: idx 3,4 → e4,e5.
	// row0: e1, e4.  row1: e2, e5.  row2: e3 (no second entry).
	entries := []string{"e1", "e2", "e3", "e4", "e5"}
	got := Columns(entries, 8)
	// 2 cols: 2+2+2 = 6 ≤ 8 → fits. 3 cols: rows=2, col0=[0,1], col1=[2,3], col2=[4].
	// widths: 2+2+2+2*2 = 10 > 8 → doesn't fit. So 2 cols.
	want := [][]string{{"e1", "e4"}, {"e2", "e5"}, {"e3"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns 5 entries width 8 = %v, want %v", got, want)
	}
}

func TestColumnsVaryingWidths(t *testing.T) {
	t.Parallel()
	// Per-column width computation (R1.4): column widths are from the longest
	// entry in each column, not the global maximum.
	entries := []string{"short", "a", "longer-entry", "b"}
	// 2 cols, rows=2: col0=[0,1]=["short","a"]→maxW=5. col1=[2,3]=["longer-entry","b"]→maxW=12.
	// total = 5 + 2 + 12 = 19.
	// 4 cols, rows=1: col0=[0]="short"→5. col1=[1]="a"→1. col2=[2]="longer-entry"→12. col3=[3]="b"→1.
	// total = 5+2+1+2+12+2+1 = 25.
	got := Columns(entries, 25)
	// 4 cols fit: single row.
	want := [][]string{{"short", "a", "longer-entry", "b"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns varying widths = %v, want %v", got, want)
	}

	// Now narrow to force 2 columns.
	got = Columns(entries, 20)
	want = [][]string{{"short", "longer-entry"}, {"a", "b"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns varying widths narrow = %v, want %v", got, want)
	}
}

func TestColumnsUnicodeRuneWidth(t *testing.T) {
	t.Parallel()
	// R1.3: column width uses rune count, not byte count.
	// "café" is 4 runes but 5 bytes (é is 2 bytes in UTF-8).
	entries := []string{"café", "test"}
	got := Columns(entries, 10)
	// Both entries are 4 runes. 2 cols: 4+2+4 = 10 ≤ 10 → fits.
	want := [][]string{{"café", "test"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns unicode = %v, want %v", got, want)
	}
}

func TestColumnsFitsWithinTermWidth(t *testing.T) {
	t.Parallel()
	// Verify that the output never exceeds termWidth for non-degenerate inputs.
	entries := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta"}
	termWidth := 30
	got := Columns(entries, termWidth)

	// Compute the display width of each row.
	for rowIdx, row := range got {
		totalWidth := 0
		for colIdx, entry := range row {
			totalWidth += len(entry)
			if colIdx < len(row)-1 {
				totalWidth += columnGap
			}
		}
		if totalWidth > termWidth {
			t.Errorf("row %d display width %d exceeds termWidth %d: %v", rowIdx, totalWidth, termWidth, row)
		}
	}
}
