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
		{"pad short string", "ab", 5, "ab   "},
		{"exact width", "abcde", 5, "abcde"},
		{"longer than width", "abcdef", 5, "abcdef"},
		{"empty string", "", 3, "   "},
		{"zero width", "ab", 0, "ab"},
		{"single char", "x", 4, "x   "},
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
		{"pad short string", "ab", 5, "   ab"},
		{"exact width", "abcde", 5, "abcde"},
		{"longer than width", "abcdef", 5, "abcdef"},
		{"empty string", "", 3, "   "},
		{"zero width", "ab", 0, "ab"},
		{"single char", "x", 4, "   x"},
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
