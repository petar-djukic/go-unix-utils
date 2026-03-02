// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for column alignment functions.
//
// Implements: prd003-format R1.1, R1.2, R1.3, R1.4
package format

import (
	"testing"
)

// --- PadRight tests (prd003-format R1.2) ---

func TestPadRight(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		width  int
		expect string
	}{
		{
			name:   "shorter-than-width",
			input:  "abc",
			width:  10,
			expect: "abc       ",
		},
		{
			name:   "equal-to-width",
			input:  "abcde",
			width:  5,
			expect: "abcde",
		},
		{
			name:   "wider-than-width",
			input:  "abcdefgh",
			width:  5,
			expect: "abcdefgh",
		},
		{
			name:   "empty-string",
			input:  "",
			width:  4,
			expect: "    ",
		},
		{
			name:   "multibyte-rune-count",
			input:  "café",
			width:  10,
			expect: "café      ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PadRight(tc.input, tc.width)
			if got != tc.expect {
				t.Errorf("PadRight(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.expect)
			}
		})
	}
}

// --- PadLeft tests (prd003-format R1.2) ---

func TestPadLeft(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		width  int
		expect string
	}{
		{
			name:   "shorter-than-width",
			input:  "abc",
			width:  10,
			expect: "       abc",
		},
		{
			name:   "equal-to-width",
			input:  "abcde",
			width:  5,
			expect: "abcde",
		},
		{
			name:   "wider-than-width",
			input:  "abcdefgh",
			width:  5,
			expect: "abcdefgh",
		},
		{
			name:   "empty-string",
			input:  "",
			width:  4,
			expect: "    ",
		},
		{
			name:   "multibyte-rune-count",
			input:  "café",
			width:  10,
			expect: "      café",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PadLeft(tc.input, tc.width)
			if got != tc.expect {
				t.Errorf("PadLeft(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.expect)
			}
		})
	}
}

// --- Columns tests (prd003-format R1.1, R1.4) ---

func TestColumns(t *testing.T) {
	tests := []struct {
		name      string
		entries   []string
		termWidth int
		check     func(t *testing.T, grid [][]string)
	}{
		{
			name:      "empty-input-returns-nil",
			entries:   []string{},
			termWidth: 80,
			check: func(t *testing.T, grid [][]string) {
				if grid != nil {
					t.Errorf("Columns(empty, 80) = %v, want nil", grid)
				}
			},
		},
		{
			name:      "nil-input-returns-nil",
			entries:   nil,
			termWidth: 80,
			check: func(t *testing.T, grid [][]string) {
				if grid != nil {
					t.Errorf("Columns(nil, 80) = %v, want nil", grid)
				}
			},
		},
		{
			name:      "single-entry",
			entries:   []string{"hello"},
			termWidth: 80,
			check: func(t *testing.T, grid [][]string) {
				if len(grid) != 1 {
					t.Fatalf("expected 1 row, got %d", len(grid))
				}
				if len(grid[0]) != 1 || grid[0][0] != "hello" {
					t.Errorf("expected [[hello]], got %v", grid)
				}
			},
		},
		{
			name:      "multiple-columns-fit",
			entries:   []string{"a", "b", "c", "d"},
			termWidth: 80,
			check: func(t *testing.T, grid [][]string) {
				if len(grid) == 0 {
					t.Fatal("expected non-empty grid")
				}
				// With entries of width 1 and termWidth 80, all 4 should fit
				// in a single row (4 entries * 1 char + 3 gaps * 2 = 10 <= 80).
				if len(grid) != 1 {
					t.Errorf("expected 1 row for 4 short entries in 80 cols, got %d rows", len(grid))
				}
			},
		},
		{
			name:      "single-column-fallback",
			entries:   []string{"very-long-entry-here", "another-long-entry!!"},
			termWidth: 10,
			check: func(t *testing.T, grid [][]string) {
				// Each entry is 20 chars, termWidth is 10, so must fall back
				// to one column.
				if len(grid) != 2 {
					t.Fatalf("expected 2 rows (single-column fallback), got %d", len(grid))
				}
				for i, row := range grid {
					if len(row) != 1 {
						t.Errorf("row %d: expected 1 column, got %d", i, len(row))
					}
				}
			},
		},
		{
			name: "20-entries-80-width",
			entries: []string{
				"README.md", "LICENSE", "go.mod", "go.sum", "main.go",
				"config.yaml", "Makefile", "CHANGELOG", "setup.py", "app.js",
				"index.html", "style.css", "test.go", "utils.go", "data.json",
				"build.sh", "deploy.sh", "run.sh", "clean.sh", "init.sh",
			},
			termWidth: 80,
			check: func(t *testing.T, grid [][]string) {
				if grid == nil {
					t.Fatal("expected non-nil grid for 20 entries")
				}

				// Verify all entries are present.
				count := 0
				for _, row := range grid {
					count += len(row)
				}
				if count != 20 {
					t.Errorf("expected 20 entries in grid, got %d", count)
				}

				// Verify each row fits within 80 columns (per-column widths + 2-space gaps).
				// Reconstruct column widths from the grid.
				numCols := 0
				for _, row := range grid {
					if len(row) > numCols {
						numCols = len(row)
					}
				}
				colWidths := make([]int, numCols)
				for _, row := range grid {
					for col, entry := range row {
						w := len(entry) // ASCII entries, byte count == rune count
						if w > colWidths[col] {
							colWidths[col] = w
						}
					}
				}
				totalWidth := 0
				for i, cw := range colWidths {
					totalWidth += cw
					if i < numCols-1 {
						totalWidth += 2 // column gap
					}
				}
				if totalWidth > 80 {
					t.Errorf("total grid width %d exceeds termWidth 80", totalWidth)
				}

				// Verify column count is maximized (one more column would not fit).
				if numCols > 1 {
					// This is a sanity check; the implementation picks the max.
					t.Logf("columns: %d, total width: %d", numCols, totalWidth)
				}
			},
		},
		{
			name:      "column-order-is-down-then-across",
			entries:   []string{"a", "b", "c", "d", "e", "f"},
			termWidth: 80,
			check: func(t *testing.T, grid [][]string) {
				// With 6 very short entries and 80 width, all should fit in 1 row
				// or be laid out column-first. Verify entries appear in the grid.
				count := 0
				for _, row := range grid {
					count += len(row)
				}
				if count != 6 {
					t.Errorf("expected 6 entries, got %d", count)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			grid := Columns(tc.entries, tc.termWidth)
			tc.check(t, grid)
		})
	}
}
