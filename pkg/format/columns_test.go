// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"fmt"
	"strings"
	"testing"
)

func TestDisplayWidth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "hello", 5},
		{"multibyte", "héllo", 5},
		{"pure unicode", "日本語", 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := displayWidth(tc.s); got != tc.want {
				t.Errorf("displayWidth(%q) = %d, want %d", tc.s, got, tc.want)
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{"pad to 10", "abc", 10, "abc       "},
		{"already exact", "hello", 5, "hello"},
		{"already wider", "toolong", 3, "toolong"},
		{"empty string", "", 4, "    "},
		{"width zero", "abc", 0, "abc"},
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
		{"pad to 10", "abc", 10, "       abc"},
		{"already exact", "hello", 5, "hello"},
		{"already wider", "toolong", 3, "toolong"},
		{"empty string", "", 4, "    "},
		{"width zero", "abc", 0, "abc"},
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

func TestColumns_empty(t *testing.T) {
	t.Parallel()
	if got := Columns(nil, 80); got != nil {
		t.Errorf("Columns(nil, 80) = %v, want nil", got)
	}
	if got := Columns([]string{}, 80); got != nil {
		t.Errorf("Columns([], 80) = %v, want nil", got)
	}
}

func TestColumns_singleEntry(t *testing.T) {
	t.Parallel()
	grid := Columns([]string{"hello"}, 80)
	if len(grid) != 1 || len(grid[0]) != 1 {
		t.Fatalf("Columns([\"hello\"], 80) = %v, want 1×1 grid", grid)
	}
	// Single entry needs no padding beyond itself.
	if got := strings.TrimRight(grid[0][0], " "); got != "hello" {
		t.Errorf("cell = %q, want \"hello\"", got)
	}
}

// TestColumns_rowsFitWidth verifies that Columns(20 filenames, 80) produces
// a grid where each row fits within 80 characters (use case S1).
func TestColumns_rowsFitWidth(t *testing.T) {
	t.Parallel()
	entries := make([]string, 20)
	for i := range entries {
		entries[i] = fmt.Sprintf("file%d", i+1)
	}

	termWidth := 80
	grid := Columns(entries, termWidth)

	if len(grid) == 0 {
		t.Fatal("Columns returned an empty grid")
	}

	// Every row must fit within termWidth when joined with the two-space separator.
	for r, row := range grid {
		line := strings.Join(row, "  ")
		if len(line) > termWidth {
			t.Errorf("row %d exceeds %d chars: len=%d, row=%q", r, termWidth, len(line), line)
		}
	}

	// All entries must appear in the grid.
	seen := make(map[string]bool)
	for _, row := range grid {
		for _, cell := range row {
			seen[strings.TrimRight(cell, " ")] = true
		}
	}
	for _, entry := range entries {
		if !seen[entry] {
			t.Errorf("entry %q missing from grid", entry)
		}
	}
}

// TestColumns_uniformColumnWidths verifies that cells in the same column share
// a uniform width (AC2).
func TestColumns_uniformColumnWidths(t *testing.T) {
	t.Parallel()
	entries := []string{"a", "bb", "ccc", "dd", "eeeee", "f"}
	grid := Columns(entries, 80)

	if len(grid) == 0 {
		t.Fatal("Columns returned an empty grid")
	}
	numCols := len(grid[0])
	colWidths := make([]int, numCols)
	for c := range colWidths {
		// Find the width of the first cell in this column.
		colWidths[c] = len(grid[0][c])
	}
	// All subsequent rows must have the same cell widths in each column.
	for r, row := range grid[1:] {
		for c, cell := range row {
			if len(cell) != colWidths[c] {
				t.Errorf("row %d col %d: cell width %d, want %d (cell=%q)",
					r+1, c, len(cell), colWidths[c], cell)
			}
		}
	}
}

// TestColumns_narrowTerminal verifies that a terminal width of 1 forces
// a single-column layout.
func TestColumns_narrowTerminal(t *testing.T) {
	t.Parallel()
	entries := []string{"a", "b", "c"}
	grid := Columns(entries, 1)
	for _, row := range grid {
		if len(row) != 1 {
			t.Errorf("expected single column at width=1, got %d columns", len(row))
		}
	}
}
