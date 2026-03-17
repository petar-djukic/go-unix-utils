// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"reflect"
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
		{"shorter than width", "abc", 6, "abc   "},
		{"exact width", "abc", 3, "abc"},
		{"longer than width", "abcdef", 3, "abcdef"},
		{"empty string", "", 5, "     "},
		{"zero width", "abc", 0, "abc"},
		{"single char", "x", 4, "x   "},
		// R1.3: Unicode rune count, not byte count.
		{"unicode runes", "café", 6, "café  "},
		{"unicode at width", "héllo", 5, "héllo"},
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
		{"shorter than width", "abc", 6, "   abc"},
		{"exact width", "abc", 3, "abc"},
		{"longer than width", "abcdef", 3, "abcdef"},
		{"empty string", "", 5, "     "},
		{"zero width", "abc", 0, "abc"},
		{"single char", "x", 4, "   x"},
		// R1.3: Unicode rune count, not byte count.
		{"unicode runes", "café", 6, "  café"},
		{"unicode at width", "héllo", 5, "héllo"},
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

func TestColumns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		entries   []string
		termWidth int
		want      [][]string
	}{
		{
			name:      "empty entries",
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
			name:      "two entries fit in two columns",
			entries:   []string{"ab", "cd"},
			termWidth: 10,
			want:      [][]string{{"ab", "cd"}},
		},
		{
			name:      "entries too wide for two columns",
			entries:   []string{"abcdefgh", "ijklmnop"},
			termWidth: 10,
			want:      [][]string{{"abcdefgh"}, {"ijklmnop"}},
		},
		{
			name:    "column-first fill order",
			entries: []string{"a", "b", "c", "d", "e", "f"},
			// With termWidth=20 and 1-char entries, 6 cols need 6+5*2=16 <= 20.
			termWidth: 20,
			// All 6 entries in one row since they all fit.
			want: [][]string{{"a", "b", "c", "d", "e", "f"}},
		},
		{
			name:    "column-first fill with two rows",
			entries: []string{"a", "b", "c", "d", "e", "f"},
			// With termWidth=10: 6 cols need 1*6+2*5=16 > 10, 5 cols need 1*5+2*4=13 > 10,
			// 4 cols need 1*4+2*3=10 <= 10. 4 cols, 2 rows.
			// Col 0: a,b  Col 1: c,d  Col 2: e,f  Col 3: (empty partial)
			// Actually with 4 cols, numRows = ceil(6/4) = 2
			// Col 0 (idx 0,1): a,b  Col 1 (idx 2,3): c,d  Col 2 (idx 4,5): e,f  Col 3 (idx 6,7): -,-
			termWidth: 10,
			want:      [][]string{{"a", "c", "e"}, {"b", "d", "f"}},
		},
		{
			name:      "entries wider than termWidth fall back to single column",
			entries:   []string{"verylongentry", "anotherlongone"},
			termWidth: 5,
			want:      [][]string{{"verylongentry"}, {"anotherlongone"}},
		},
		{
			name:    "per-column width calculation",
			entries: []string{"short", "x", "longentry", "y"},
			// 2 cols, 2 rows: col0 has "short","x" (max 5), col1 has "longentry","y" (max 9)
			// Total: 5 + 2 + 9 = 16
			termWidth: 16,
			want:      [][]string{{"short", "longentry"}, {"x", "y"}},
		},
		{
			name:    "per-column width blocks extra columns",
			entries: []string{"short", "x", "longentry", "y"},
			// 4 cols, 1 row: col widths 5+2+9+2+1 = too much for 15
			// 3 cols: ceil(4/3)=2 rows. col0: short,x (5); col1: longentry,y (9); col2: (empty partial)
			// Hmm: col0 idx 0,1: short,x; col1 idx 2,3: longentry,y; col2 idx 4,5: -,-
			// That's only 2 cols used. Let me recompute.
			// 3 cols, numRows=2: col0(idx 0,1)=short,x; col1(idx 2,3)=longentry,y; col2(idx 4,5)=none
			// widths: 5 + 2 + 9 = 16 > 15. Doesn't fit.
			// 2 cols, numRows=2: col0(idx 0,1)=short,x (5); col1(idx 2,3)=longentry,y (9); total=5+2+9=16 > 15
			// 1 col.
			termWidth: 15,
			want:      [][]string{{"short"}, {"x"}, {"longentry"}, {"y"}},
		},
		{
			name:    "three columns with uneven distribution",
			entries: []string{"aa", "bb", "cc", "dd", "ee"},
			// 3 cols, numRows=2: col0(0,1)=aa,bb(2); col1(2,3)=cc,dd(2); col2(4)=ee(2)
			// total: 2+2+2+2+2=10. fits in 10.
			termWidth: 10,
			want:      [][]string{{"aa", "cc", "ee"}, {"bb", "dd"}},
		},
		{
			name:    "unicode entries column width",
			entries: []string{"café", "naïve", "résumé"},
			// Display widths: 4, 5, 6. Single row needs 4+2+5+2+6=19.
			termWidth: 20,
			want:      [][]string{{"café", "naïve", "résumé"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Columns(tc.entries, tc.termWidth)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Columns(%v, %d) =\n  %v\nwant:\n  %v", tc.entries, tc.termWidth, got, tc.want)
			}
		})
	}
}
