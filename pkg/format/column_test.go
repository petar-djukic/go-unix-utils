// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd003-format R1.1-R1.4: Columns, PadRight, PadLeft, displayWidth.

package format_test

import (
	"reflect"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
)

// TestColumns_EmptySlice verifies that Columns returns nil for an empty input.
// R1.1: zero entries produce no grid.
func TestColumns_EmptySlice(t *testing.T) {
	t.Parallel()

	got := format.Columns(nil, 80)
	if got != nil {
		t.Errorf("Columns(nil, 80) = %v, want nil", got)
	}

	got = format.Columns([]string{}, 80)
	if got != nil {
		t.Errorf("Columns([]string{}, 80) = %v, want nil", got)
	}
}

// TestColumns_SingleEntry verifies that a single entry produces a one-row,
// one-column grid regardless of terminal width.
// R1.1: single entry always fits in one column.
func TestColumns_SingleEntry(t *testing.T) {
	t.Parallel()

	got := format.Columns([]string{"hello"}, 80)
	want := [][]string{{"hello"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns([hello], 80) = %v, want %v", got, want)
	}
}

// TestColumns_RowMajorOrder verifies that entries are placed left-to-right,
// top-to-bottom (row-major order).
// R1.1, R1.4: row-major distribution across columns.
func TestColumns_RowMajorOrder(t *testing.T) {
	t.Parallel()

	// Four short entries with a wide terminal should fit in one row.
	entries := []string{"a", "b", "c", "d"}
	// Each entry is 1 char wide. 4 columns: 1 + 2 + 1 + 2 + 1 + 2 + 1 = 10
	// (3 gaps of 2 spaces between 4 columns = 6, plus 4 chars = 10).
	got := format.Columns(entries, 80)
	// Should be a single row with all entries.
	want := [][]string{{"a", "b", "c", "d"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns([a,b,c,d], 80) = %v, want %v", got, want)
	}
}

// TestColumns_MultipleRows verifies distribution across multiple rows when
// entries do not all fit on one row.
// R1.1, R1.4: maximise columns, row-major layout.
func TestColumns_MultipleRows(t *testing.T) {
	t.Parallel()

	// 6 entries, each 8 chars wide. With columnGap=2, each column costs 10
	// chars (except the last which costs 8). 3 columns: 10+10+8=28, fits in 30.
	// 4 columns: 10+10+10+8=38, does not fit in 30.
	entries := []string{"aaaaaaaa", "bbbbbbbb", "cccccccc", "dddddddd", "eeeeeeee", "ffffffff"}
	got := format.Columns(entries, 30)
	// 3 columns, 2 rows, row-major:
	// Row 0: entries[0], entries[1], entries[2]
	// Row 1: entries[3], entries[4], entries[5]
	want := [][]string{
		{"aaaaaaaa", "bbbbbbbb", "cccccccc"},
		{"dddddddd", "eeeeeeee", "ffffffff"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns(6 entries, 30) = %v, want %v", got, want)
	}
}

// TestColumns_SingleColumnFallback verifies that entries too wide for two
// columns fall back to single-column layout.
// R1.1: single-column fallback when no multi-column layout fits.
func TestColumns_SingleColumnFallback(t *testing.T) {
	t.Parallel()

	// Two entries each 10 chars wide. Two columns need 10+2+10=22 chars.
	// With termWidth=20, two columns do not fit; single-column fallback.
	entries := []string{"aaaaaaaaaa", "bbbbbbbbbb"}
	got := format.Columns(entries, 20)
	want := [][]string{
		{"aaaaaaaaaa"},
		{"bbbbbbbbbb"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns(2 wide entries, 20) = %v, want %v", got, want)
	}
}

// TestColumns_ExactFit verifies that entries exactly filling the terminal
// width use the maximum number of columns.
// R1.1, R1.4: per-column width with exact fit.
func TestColumns_ExactFit(t *testing.T) {
	t.Parallel()

	// 3 entries: widths 5, 5, 5. Three columns: 5+2+5+2+5=19.
	entries := []string{"abcde", "fghij", "klmno"}
	got := format.Columns(entries, 19)
	// Should fit in one row of 3 columns.
	want := [][]string{{"abcde", "fghij", "klmno"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns(3x5-char, 19) = %v, want %v", got, want)
	}
}

// TestColumns_PerColumnWidth verifies that column widths are determined by
// the longest entry in each column, not the global maximum.
// R1.4: per-column max display widths.
func TestColumns_PerColumnWidth(t *testing.T) {
	t.Parallel()

	// 4 entries with varying widths. Row-major with 2 columns:
	// Col 0: max(len("ab"), len("abcdef")) = 6
	// Col 1: max(len("cde"), len("gh")) = 3
	// Total: 6 + 2 + 3 = 11, fits in termWidth=11.
	// With 4 columns: 2+2+3+2+6+2+2 = too wide — try 3 columns too.
	// 3 columns (2 rows): Row0=[ab, cde, abcdef], Row1=[gh]
	// Col0: max(2,2)=2, Col1: max(3,0)=3, Col2: max(6,0)=6 => 2+2+3+2+6=15 > 11.
	// 2 columns: Col0: max(2,6)=6, Col1: max(3,2)=3 => 6+2+3=11 fits.
	entries := []string{"ab", "cde", "abcdef", "gh"}
	got := format.Columns(entries, 11)
	want := [][]string{
		{"ab", "cde"},
		{"abcdef", "gh"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns(varying widths, 11) = %v, want %v", got, want)
	}
}

// TestColumns_UnevenLastRow verifies that the last row may have fewer entries
// than other rows when entries don't divide evenly by column count.
// R1.1: incomplete last row.
func TestColumns_UnevenLastRow(t *testing.T) {
	t.Parallel()

	// 5 entries, each 3 chars. 4 columns: 3+2+3+2+3+2+3=18 fits in 18.
	// Columns maximizes columns, so 4 columns with 2 rows:
	// Row 0: entries[0..3] = [aaa, bbb, ccc, ddd]
	// Row 1: entries[4]    = [eee]
	entries := []string{"aaa", "bbb", "ccc", "ddd", "eee"}
	got := format.Columns(entries, 18)
	want := [][]string{
		{"aaa", "bbb", "ccc", "ddd"},
		{"eee"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns(5x3-char, 18) = %v, want %v", got, want)
	}
}

// TestPadRight verifies left-aligned padding with trailing spaces.
// R1.2: PadRight pads with trailing spaces.
func TestPadRight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"shorter_than_width", "abc", 6, "abc   "},
		{"equal_to_width", "abcdef", 6, "abcdef"},
		{"longer_than_width", "abcdefgh", 6, "abcdefgh"},
		{"empty_string", "", 4, "    "},
		{"zero_width", "abc", 0, "abc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := format.PadRight(tc.input, tc.width)
			if got != tc.want {
				t.Errorf("PadRight(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
			}
		})
	}
}

// TestPadLeft verifies right-aligned padding with leading spaces.
// R1.2: PadLeft pads with leading spaces.
func TestPadLeft(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"shorter_than_width", "abc", 6, "   abc"},
		{"equal_to_width", "abcdef", 6, "abcdef"},
		{"longer_than_width", "abcdefgh", 6, "abcdefgh"},
		{"empty_string", "", 4, "    "},
		{"zero_width", "abc", 0, "abc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := format.PadLeft(tc.input, tc.width)
			if got != tc.want {
				t.Errorf("PadLeft(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
			}
		})
	}
}

// TestPadRight_MultiByte verifies that PadRight uses rune count, not byte
// count, for multi-byte UTF-8 strings.
// R1.2, R1.3: padding is based on display width (rune count).
func TestPadRight_MultiByte(t *testing.T) {
	t.Parallel()

	// "café" is 4 runes but 5 bytes (é is 2 bytes in UTF-8).
	got := format.PadRight("café", 8)
	// 4 runes + 4 spaces = 8 display width.
	want := "café    "
	if got != want {
		t.Errorf("PadRight(%q, 8) = %q, want %q", "café", got, want)
	}
}

// TestPadLeft_MultiByte verifies that PadLeft uses rune count, not byte
// count, for multi-byte UTF-8 strings.
// R1.2, R1.3: padding is based on display width (rune count).
func TestPadLeft_MultiByte(t *testing.T) {
	t.Parallel()

	// "日本" is 2 runes but 6 bytes.
	got := format.PadLeft("日本", 5)
	// 2 runes + 3 leading spaces = 5 display width.
	want := "   日本"
	if got != want {
		t.Errorf("PadLeft(%q, 5) = %q, want %q", "日本", got, want)
	}
}

// TestColumns_MultiByte verifies that Columns computes column widths using
// rune count for multi-byte UTF-8 entries.
// R1.1, R1.3, R1.4: column layout with rune-based width computation.
func TestColumns_MultiByte(t *testing.T) {
	t.Parallel()

	// "café" = 4 runes, "ab" = 2 runes.
	// Two columns: col0 max=4, col1 max=2 => 4+2+2=8. Fits in 10.
	entries := []string{"café", "ab"}
	got := format.Columns(entries, 10)
	want := [][]string{{"café", "ab"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns([café, ab], 10) = %v, want %v", got, want)
	}
}

// TestColumns_NarrowTerminal verifies that when the terminal is too narrow
// for even two columns, every entry gets its own row.
// R1.1: single-column fallback for narrow terminals.
func TestColumns_NarrowTerminal(t *testing.T) {
	t.Parallel()

	entries := []string{"hello", "world", "foo"}
	// Two columns would need at minimum 5+2+5=12, won't fit in 8.
	got := format.Columns(entries, 8)
	want := [][]string{
		{"hello"},
		{"world"},
		{"foo"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns(3 entries, 8) = %v, want %v", got, want)
	}
}
