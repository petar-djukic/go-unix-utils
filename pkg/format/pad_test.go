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
		{"shorter", "ab", 5, "ab   "},
		{"equal", "abcde", 5, "abcde"},
		{"longer", "abcdef", 5, "abcdef"},
		{"empty", "", 3, "   "},
		{"unicode", "café", 6, "café  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := PadRight(tt.s, tt.width); got != tt.want {
				t.Errorf("PadRight(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
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
		{"equal", "abcde", 5, "abcde"},
		{"longer", "abcdef", 5, "abcdef"},
		{"empty", "", 3, "   "},
		{"unicode", "café", 6, "  café"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := PadLeft(tt.s, tt.width); got != tt.want {
				t.Errorf("PadLeft(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
			}
		})
	}
}

func TestColumnsEmpty(t *testing.T) {
	t.Parallel()
	if got := Columns(nil, 80); got != nil {
		t.Errorf("Columns(nil, 80) = %v, want nil", got)
	}
}

func TestColumnsSingle(t *testing.T) {
	t.Parallel()
	got := Columns([]string{"hello"}, 80)
	want := [][]string{{"hello"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns([hello], 80) = %v, want %v", got, want)
	}
}

func TestColumnsMulti(t *testing.T) {
	t.Parallel()
	entries := []string{"a", "b", "c", "d"}
	got := Columns(entries, 4)
	want := [][]string{{"a", "c"}, {"b", "d"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns(%v, 4) = %v, want %v", entries, got, want)
	}
}

func TestColumnsForceSingleColumn(t *testing.T) {
	t.Parallel()
	entries := []string{"hello", "world"}
	got := Columns(entries, 3)
	want := [][]string{{"hello"}, {"world"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Columns(%v, 3) = %v, want %v", entries, got, want)
	}
}
