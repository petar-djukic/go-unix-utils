// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "testing"

func TestPadRight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		// AC6: basic padding.
		{"pad short", "abc", 6, "abc   "},
		{"exact width", "abcdef", 6, "abcdef"},
		{"over width", "abcdefgh", 6, "abcdefgh"},
		{"empty string", "", 4, "    "},
		{"zero width", "abc", 0, "abc"},
		// R1.3: multi-byte characters counted by rune, not byte.
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
		// AC6: basic padding.
		{"pad short", "abc", 6, "   abc"},
		{"exact width", "abcdef", 6, "abcdef"},
		{"over width", "abcdefgh", 6, "abcdefgh"},
		{"empty string", "", 4, "    "},
		{"zero width", "abc", 0, "abc"},
		// R1.3: multi-byte characters counted by rune, not byte.
		{"unicode", "café", 6, "  café"},
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
