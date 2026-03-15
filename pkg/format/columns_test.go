// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPadRight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"shorter than width", "abc", 6, "abc   "},
		{"exact width", "abc", 3, "abc"},
		{"longer than width", "abcdef", 3, "abcdef"},
		{"empty string", "", 5, "     "},
		{"zero width", "abc", 0, "abc"},
		{"negative width", "abc", -1, "abc"},
		{"width one", "a", 1, "a"},
		{"pad by one", "ab", 3, "ab "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PadRight(tc.input, tc.width)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPadLeft(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"shorter than width", "abc", 6, "   abc"},
		{"exact width", "abc", 3, "abc"},
		{"longer than width", "abcdef", 3, "abcdef"},
		{"empty string", "", 5, "     "},
		{"zero width", "abc", 0, "abc"},
		{"negative width", "abc", -1, "abc"},
		{"width one", "a", 1, "a"},
		{"pad by one", "ab", 3, " ab"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PadLeft(tc.input, tc.width)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestColumns(t *testing.T) {
	t.Parallel()

	t.Run("empty input", func(t *testing.T) {
		t.Parallel()
		result := Columns(nil, 80)
		assert.Nil(t, result)

		result = Columns([]string{}, 80)
		assert.Nil(t, result)
	})

	t.Run("single entry", func(t *testing.T) {
		t.Parallel()
		result := Columns([]string{"hello"}, 80)
		require.Len(t, result, 1)
		assert.Equal(t, []string{"hello"}, result[0])
	})

	t.Run("zero termWidth falls back to single column", func(t *testing.T) {
		t.Parallel()
		entries := []string{"a", "b", "c"}
		result := Columns(entries, 0)
		require.Len(t, result, 3)
		assert.Equal(t, []string{"a"}, result[0])
		assert.Equal(t, []string{"b"}, result[1])
		assert.Equal(t, []string{"c"}, result[2])
	})

	t.Run("negative termWidth falls back to single column", func(t *testing.T) {
		t.Parallel()
		entries := []string{"a", "b", "c"}
		result := Columns(entries, -5)
		require.Len(t, result, 3)
		for i, e := range entries {
			assert.Equal(t, []string{e}, result[i])
		}
	})

	t.Run("entry exceeds termWidth falls back to single column", func(t *testing.T) {
		t.Parallel()
		entries := []string{"short", "this-is-a-very-long-entry-that-exceeds-width", "x"}
		result := Columns(entries, 10)
		require.Len(t, result, 3)
		for i, e := range entries {
			assert.Equal(t, []string{e}, result[i])
		}
	})

	t.Run("column-first fill order", func(t *testing.T) {
		t.Parallel()
		// 5 entries in 2 columns => 3 rows
		// Col 0: a, b, c  Col 1: d, e
		entries := []string{"a", "b", "c", "d", "e"}
		result := Columns(entries, 80)
		// With short entries and wide terminal, could fit many columns.
		// But the column-first order should be preserved.
		// Verify that entries appear in column-first order across rows.
		var recovered []string
		if len(result) > 0 {
			numCols := len(result[0])
			rows := len(result)
			for col := 0; col < numCols; col++ {
				for row := 0; row < rows; row++ {
					if col < len(result[row]) {
						recovered = append(recovered, result[row][col])
					}
				}
			}
		}
		assert.Equal(t, entries, recovered)
	})

	t.Run("two columns with gap", func(t *testing.T) {
		t.Parallel()
		// Entries: "aaaa" (4), "bb" (2), "cccc" (4), "dd" (2)
		// termWidth = 10
		// 2 cols, 2 rows: col0=[aaaa,bb] width=4, col1=[cccc,dd] width=4
		// total = 4 + 2 + 4 = 10 => fits
		// 4 cols, 1 row: col0=[aaaa] w=4, col1=[bb] w=2, col2=[cccc] w=4, col3=[dd] w=2
		// total = 4+2+2+2+2+2+4+2 = wait, 4+2+2+2+4+2 hmm
		// total = 4 + 2(gap) + 2 + 2(gap) + 4 + 2(gap) + 2 = 18 => doesn't fit
		// 3 cols: not possible to divide 4 into 3 evenly
		// Actually ceil(4/3)=2 rows. col0=[aaaa,bb] w=4, col1=[cccc,dd] w=4, col2=[] empty
		// Hmm, with 3 cols and 2 rows: col0=[e[0],e[1]]=[aaaa,bb], col1=[e[2],e[3]]=[cccc,dd], col2 starts at idx 4 which is >= len
		// So effectively 2 cols. The algo tries 4, 3, 2 cols.
		entries := []string{"aaaa", "bb", "cccc", "dd"}
		result := Columns(entries, 10)
		require.Len(t, result, 2)
		// Row 0: aaaa, cccc  Row 1: bb, dd
		assert.Equal(t, []string{"aaaa", "cccc"}, result[0])
		assert.Equal(t, []string{"bb", "dd"}, result[1])
	})

	t.Run("all entries fit in one row", func(t *testing.T) {
		t.Parallel()
		entries := []string{"a", "b", "c"}
		// Each entry is 1 char, gaps = 2*2 = 4, total = 1+2+1+2+1 = 7
		result := Columns(entries, 80)
		require.Len(t, result, 1)
		assert.Equal(t, []string{"a", "b", "c"}, result[0])
	})

	t.Run("exact fit at termWidth boundary", func(t *testing.T) {
		t.Parallel()
		// Two entries of 4 chars each. 2 cols: 4 + 2(gap) + 4 = 10
		entries := []string{"abcd", "efgh"}
		result := Columns(entries, 10)
		require.Len(t, result, 1)
		assert.Equal(t, []string{"abcd", "efgh"}, result[0])
	})

	t.Run("does not fit in two columns", func(t *testing.T) {
		t.Parallel()
		// Two entries of 5 chars each. 2 cols: 5 + 2(gap) + 5 = 12
		entries := []string{"abcde", "fghij"}
		result := Columns(entries, 11)
		require.Len(t, result, 2)
		assert.Equal(t, []string{"abcde"}, result[0])
		assert.Equal(t, []string{"fghij"}, result[1])
	})

	t.Run("per-column width not global max", func(t *testing.T) {
		t.Parallel()
		// R1.4: column widths from longest entry in each column, not global max.
		// entries: "longname" (8), "a" (1), "b" (1), "c" (1)
		// 2 cols, 2 rows: col0=[longname, a] w=8, col1=[b, c] w=1
		// total = 8 + 2 + 1 = 11 => fits in 11
		entries := []string{"longname", "a", "b", "c"}
		result := Columns(entries, 11)
		require.Len(t, result, 2)
		assert.Equal(t, []string{"longname", "b"}, result[0])
		assert.Equal(t, []string{"a", "c"}, result[1])
	})
}
