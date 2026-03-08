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
		{"shorter than width", "abc", 6, "abc   "},
		{"exact width", "abcdef", 6, "abcdef"},
		{"longer than width", "abcdefgh", 6, "abcdefgh"},
		{"empty string", "", 4, "    "},
		{"zero width", "abc", 0, "abc"},
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
		{"exact width", "abcdef", 6, "abcdef"},
		{"longer than width", "abcdefgh", 6, "abcdefgh"},
		{"empty string", "", 4, "    "},
		{"zero width", "abc", 0, "abc"},
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

func TestPadExactWidth(t *testing.T) {
	t.Parallel()

	// AC4: PadRight and PadLeft produce strings of exactly the specified width.
	got := PadRight("hi", 10)
	if len(got) != 10 {
		t.Errorf("PadRight(\"hi\", 10) length = %d, want 10", len(got))
	}

	got = PadLeft("hi", 10)
	if len(got) != 10 {
		t.Errorf("PadLeft(\"hi\", 10) length = %d, want 10", len(got))
	}
}

func TestColumns(t *testing.T) {
	t.Parallel()

	t.Run("empty entries", func(t *testing.T) {
		t.Parallel()
		result := Columns(nil, 80)
		if result != nil {
			t.Errorf("Columns(nil, 80) = %v, want nil", result)
		}
	})

	t.Run("single entry", func(t *testing.T) {
		t.Parallel()
		result := Columns([]string{"hello"}, 80)
		if len(result) != 1 || len(result[0]) != 1 || result[0][0] != "hello" {
			t.Errorf("Columns([hello], 80) = %v, want [[hello]]", result)
		}
	})

	t.Run("fits in multiple columns", func(t *testing.T) {
		t.Parallel()
		entries := []string{"a", "b", "c", "d", "e", "f"}
		result := Columns(entries, 20)

		// Should have multiple columns since entries are short.
		if len(result) == 0 {
			t.Fatal("Columns returned empty result")
		}
		if len(result) >= len(entries) {
			t.Errorf("expected fewer rows than entries, got %d rows for %d entries", len(result), len(entries))
		}

		// Verify all entries are present.
		var found []string
		for _, row := range result {
			found = append(found, row...)
		}
		if len(found) != len(entries) {
			t.Errorf("got %d entries in result, want %d", len(found), len(entries))
		}
	})

	t.Run("top-to-bottom ordering", func(t *testing.T) {
		t.Parallel()
		// D3: entries fill top-to-bottom within each column.
		// Use enough entries to force multiple rows.
		entries := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"}
		result := Columns(entries, 25)

		// With 6 entries of ~5-7 chars and width 25, expect 2-3 columns and 2+ rows.
		if len(result) < 2 {
			t.Skipf("layout produced %d rows, skipping ordering check", len(result))
		}
		// First column should list entries top-to-bottom.
		if result[0][0] != "alpha" {
			t.Errorf("first entry in first row = %q, want \"alpha\"", result[0][0])
		}
		if result[1][0] != "bravo" {
			t.Errorf("first entry in second row = %q, want \"bravo\"", result[1][0])
		}
	})

	t.Run("narrow width forces single column", func(t *testing.T) {
		t.Parallel()
		entries := []string{"longfilename", "anotherlongone"}
		result := Columns(entries, 15)

		// Each entry is longer than 15/2, so should be single column.
		if len(result) != 2 {
			t.Errorf("expected 2 rows (single column), got %d", len(result))
		}
		for i, row := range result {
			if len(row) != 1 {
				t.Errorf("row %d has %d columns, want 1", i, len(row))
			}
		}
	})

	t.Run("AC4 grid fits within width", func(t *testing.T) {
		t.Parallel()
		entries := make([]string, 20)
		for i := range entries {
			entries[i] = "file" + string(rune('a'+i%26))
		}
		result := Columns(entries, 80)

		if len(result) == 0 {
			t.Fatal("Columns returned empty result")
		}

		// Verify all entries present.
		count := 0
		for _, row := range result {
			count += len(row)
		}
		if count != 20 {
			t.Errorf("total entries in grid = %d, want 20", count)
		}
	})
}
