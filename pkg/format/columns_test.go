// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// columns_test.go contains table-driven unit tests for Columns, PadRight,
// and PadLeft, verifying column-major layout, padding behavior, and correct
// display width computation for multi-byte UTF-8 strings.
//
// Tests: prd003-format R1.1, R1.2, R1.3, R1.4.
package format

import (
	"reflect"
	"testing"
)

func TestColumns(t *testing.T) {
	tests := []struct {
		name      string
		entries   []string
		termWidth int
		wantRows  [][]string
	}{
		{
			name:      "nil_input_returns_nil",
			entries:   nil,
			termWidth: 80,
			wantRows:  nil,
		},
		{
			name:      "empty_slice_returns_nil",
			entries:   []string{},
			termWidth: 80,
			wantRows:  nil,
		},
		{
			name:      "single_entry",
			entries:   []string{"hello"},
			termWidth: 80,
			wantRows:  [][]string{{"hello"}},
		},
		{
			name:      "entries_wider_than_terminal_single_column",
			entries:   []string{"abcdefghij", "klmnopqrst"},
			termWidth: 5,
			wantRows:  [][]string{{"abcdefghij"}, {"klmnopqrst"}},
		},
		{
			name:      "two_entries_fit_two_columns",
			entries:   []string{"aa", "bb"},
			termWidth: 80,
			wantRows:  [][]string{{"aa", "bb"}},
		},
		{
			// Column-major ordering: entries go down then across.
			// With termWidth=10, 4 cols (18) too wide, 3 cols (10) fits:
			// numRows=2, col0: [0],[1]  col1: [2],[3]  col2: empty
			// Grid: row0=[entries[0],entries[2]], row1=[entries[1],entries[3]]
			name:      "four_entries_column_major",
			entries:   []string{"aaa", "bbb", "ccc", "ddd"},
			termWidth: 10,
			wantRows:  [][]string{{"aaa", "ccc"}, {"bbb", "ddd"}},
		},
		{
			// 6 entries, termWidth=8 forces 3 columns (total width 7):
			// numRows=2, col0: [0],[1]  col1: [2],[3]  col2: [4],[5]
			name:      "six_entries_three_columns",
			entries:   []string{"a", "b", "c", "d", "e", "f"},
			termWidth: 8,
			wantRows:  [][]string{{"a", "c", "e"}, {"b", "d", "f"}},
		},
		{
			// 5 entries, termWidth=8 forces 3 columns:
			// numRows=2, col0: [0],[1]  col1: [2],[3]  col2: [4]
			name:      "five_entries_three_columns_uneven",
			entries:   []string{"a", "b", "c", "d", "e"},
			termWidth: 8,
			wantRows:  [][]string{{"a", "c", "e"}, {"b", "d"}},
		},
		{
			name:      "zero_term_width_uses_default_80",
			entries:   []string{"hello"},
			termWidth: 0,
			wantRows:  [][]string{{"hello"}},
		},
		{
			name:      "negative_term_width_uses_default_80",
			entries:   []string{"hello"},
			termWidth: -1,
			wantRows:  [][]string{{"hello"}},
		},
		{
			// Per-column width, not global max: short entries in one column
			// allow more columns than if using the global maximum.
			name:    "per_column_width_maximizes_columns",
			entries: []string{"a", "b", "longentry", "d"},
			// col-major with 2 cols, 2 rows:
			// col0: "a"(1), "b"(1) => width 1
			// col1: "longentry"(9), "d"(1) => width 9
			// total: 1 + 2(gap) + 9 = 12
			termWidth: 12,
			wantRows:  [][]string{{"a", "longentry"}, {"b", "d"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Columns(tt.entries, tt.termWidth)
			if !reflect.DeepEqual(got, tt.wantRows) {
				t.Errorf("Columns(%v, %d)\ngot:  %v\nwant: %v",
					tt.entries, tt.termWidth, got, tt.wantRows)
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{name: "ascii_pad", input: "abc", width: 10, expected: "abc       "},
		{name: "exact_width_no_pad", input: "abcde", width: 5, expected: "abcde"},
		{name: "longer_than_width_unchanged", input: "abcdefgh", width: 5, expected: "abcdefgh"},
		{name: "empty_string_pads_fully", input: "", width: 4, expected: "    "},
		// Multi-byte UTF-8: "café" is 4 runes, so needs 6 spaces to reach width 10 (R1.3).
		{name: "multibyte_utf8_rune_count", input: "café", width: 10, expected: "café      "},
		// Japanese hiragana: "あいう" is 3 runes.
		{name: "cjk_rune_count", input: "あいう", width: 6, expected: "あいう   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadRight(tt.input, tt.width)
			if got != tt.expected {
				t.Errorf("PadRight(%q, %d) = %q, want %q",
					tt.input, tt.width, got, tt.expected)
			}
		})
	}
}

func TestPadLeft(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{name: "ascii_pad", input: "abc", width: 10, expected: "       abc"},
		{name: "exact_width_no_pad", input: "abcde", width: 5, expected: "abcde"},
		{name: "longer_than_width_unchanged", input: "abcdefgh", width: 5, expected: "abcdefgh"},
		{name: "empty_string_pads_fully", input: "", width: 4, expected: "    "},
		// Multi-byte UTF-8: "café" is 4 runes (R1.3).
		{name: "multibyte_utf8_rune_count", input: "café", width: 10, expected: "      café"},
		// Japanese hiragana: "あいう" is 3 runes.
		{name: "cjk_rune_count", input: "あいう", width: 6, expected: "   あいう"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadLeft(tt.input, tt.width)
			if got != tt.expected {
				t.Errorf("PadLeft(%q, %d) = %q, want %q",
					tt.input, tt.width, got, tt.expected)
			}
		})
	}
}

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{name: "ascii", input: "hello", expected: 5},
		{name: "empty", input: "", expected: 0},
		{name: "multibyte_cafe", input: "café", expected: 4},
		{name: "cjk_three_chars", input: "あいう", expected: 3},
		{name: "mixed_ascii_multibyte", input: "aé", expected: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := displayWidth(tt.input)
			if got != tt.expected {
				t.Errorf("displayWidth(%q) = %d, want %d",
					tt.input, got, tt.expected)
			}
		})
	}
}
