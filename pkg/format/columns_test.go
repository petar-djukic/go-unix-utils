// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVisibleWidth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"plain ascii", "hello", 5},
		{"empty", "", 0},
		{"ansi colored", "\033[34mblue\033[0m", 4},
		{"multiple ansi", "\033[1m\033[32mbold green\033[0m", 10},
		{"no ansi content", "\033[34m\033[0m", 0},
		{"unicode runes", "café", 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := visibleWidth(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestStripANSI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no escapes", "hello", "hello"},
		{"single escape", "\033[34mblue\033[0m", "blue"},
		{"bold escape", "\033[1mtext\033[0m", "text"},
		{"empty string", "", ""},
		{"escape only", "\033[0m", ""},
		{"multiple params", "\033[33;1mbold yellow\033[0m", "bold yellow"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := stripANSI(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPadRight(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"shorter than width", "hi", 5, "hi   "},
		{"exact width", "hello", 5, "hello"},
		{"longer than width", "toolong", 3, "toolong"},
		{"empty string", "", 4, "    "},
		{"zero width", "hi", 0, "hi"},
		{"ansi colored shorter", "\033[34mhi\033[0m", 5, "\033[34mhi\033[0m   "},
		{"ansi colored exact", "\033[34mhello\033[0m", 5, "\033[34mhello\033[0m"},
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
		{"shorter than width", "hi", 5, "   hi"},
		{"exact width", "hello", 5, "hello"},
		{"longer than width", "toolong", 3, "toolong"},
		{"empty string", "", 4, "    "},
		{"zero width", "hi", 0, "hi"},
		{"ansi colored shorter", "\033[34mhi\033[0m", 5, "   \033[34mhi\033[0m"},
		{"ansi colored exact", "\033[34mhello\033[0m", 5, "\033[34mhello\033[0m"},
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
		got := Columns(nil, 80)
		assert.Nil(t, got)
	})

	t.Run("single entry", func(t *testing.T) {
		t.Parallel()
		got := Columns([]string{"file"}, 80)
		require.Len(t, got, 1)
		assert.Equal(t, []string{"file"}, got[0])
	})

	t.Run("column major order", func(t *testing.T) {
		t.Parallel()
		// 6 entries, termWidth=20. Each entry is 3 chars.
		// With 3 cols: 2 rows. Col widths: 3+2+3+2+3 = 13 fits in 20.
		// Column-major: col0=[a,b], col1=[c,d], col2=[e,f]
		// Row 0: a, c, e
		// Row 1: b, d, f
		entries := []string{"a", "b", "c", "d", "e", "f"}
		got := Columns(entries, 20)
		require.Len(t, got, 1) // 6 single-char entries, 6 cols: 1+2+1+2+1+2+1+2+1+2+1 = 14, fits 20
		// Actually let's verify the actual layout.
		// 6 cols, 1 row each: total = 6*1 + 5*2 = 16, fits 20.
		// Row 0: a, b, c, d, e, f
		assert.Equal(t, []string{"a", "b", "c", "d", "e", "f"}, got[0])
	})

	t.Run("column major with longer entries", func(t *testing.T) {
		t.Parallel()
		// Use entries that force 2 columns with column-major fill.
		entries := []string{"alpha", "bravo", "charlie", "delta"}
		// Widths: 5, 5, 7, 5
		// 4 cols: 5+2+5+2+7+2+5 = 28. Try termWidth=25.
		// 3 cols: 2 rows. col0=[alpha,bravo](max 5), col1=[charlie,delta](max 7), col2 is empty for row 1
		//   Actually: 3 cols, 2 rows. col0 indices [0,1], col1 indices [2,3], col2 indices [4..] = none for 4 entries
		//   Wait, numRows = ceil(4/3) = 2. col0=[0,1], col1=[2,3], col2=[4,5] -> idx 4,5 out of range.
		//   col0 max=5, col1 max=7, col2 max=0. total=5+2+7+2+0=16, fits 25.
		//   Row 0: entries[0], entries[2], (entries[4] doesn't exist) -> [alpha, charlie]
		//   Row 1: entries[1], entries[3], (entries[5] doesn't exist) -> [bravo, delta]
		// Hmm, that's only 2 entries per row with 3 cols attempted but col2 empty.
		// Actually with 4 cols: numRows=1. col widths=[5,5,7,5]. total=5+2+5+2+7+2+5=28 > 25. No fit.
		// 3 cols: numRows=2. But only 4 entries so col2 has entries[4] which is out of bounds.
		// col2 has no entries. Width = 5+2+7+2+0 = 16 fits 25.
		// Rows: [alpha, charlie], [bravo, delta]
		got := Columns(entries, 25)
		// 4 cols needs 28, won't fit in 25.
		// 3 cols needs 16, fits. row0=[alpha,charlie], row1=[bravo,delta]
		require.Len(t, got, 2)
		assert.Equal(t, []string{"alpha", "charlie"}, got[0])
		assert.Equal(t, []string{"bravo", "delta"}, got[1])
	})

	t.Run("single column fallback", func(t *testing.T) {
		t.Parallel()
		// termWidth too narrow for even 2 columns.
		entries := []string{"longname1", "longname2", "longname3"}
		// 2 cols: numRows=2. col0=[longname1,longname2](9), col1=[longname3](9). total=9+2+9=20 > 10.
		got := Columns(entries, 10)
		require.Len(t, got, 3)
		assert.Equal(t, []string{"longname1"}, got[0])
		assert.Equal(t, []string{"longname2"}, got[1])
		assert.Equal(t, []string{"longname3"}, got[2])
	})

	t.Run("ansi colored entries", func(t *testing.T) {
		t.Parallel()
		// ANSI escapes should not affect column width calculation.
		entries := []string{
			"\033[34mab\033[0m",  // visible: 2
			"\033[32mcd\033[0m",  // visible: 2
			"\033[36mef\033[0m",  // visible: 2
			"\033[35mgh\033[0m",  // visible: 2
		}
		// 4 cols: 2+2+2+2 + 3*2 = 14, fits 20.
		got := Columns(entries, 20)
		require.Len(t, got, 1)
		assert.Len(t, got[0], 4)
	})

	t.Run("per column width computation", func(t *testing.T) {
		t.Parallel()
		// Verify that column widths are per-column, not global max. R1.4.
		entries := []string{"a", "b", "cc", "longentry"}
		// 4 cols: numRows=1. widths=[1,1,2,9]. total=1+2+1+2+2+2+9=19. fits 20.
		got := Columns(entries, 20)
		require.Len(t, got, 1)
		assert.Equal(t, []string{"a", "b", "cc", "longentry"}, got[0])
	})
}
