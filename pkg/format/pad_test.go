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
