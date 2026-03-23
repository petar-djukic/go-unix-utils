// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Unit tests for pkg/format: PadRight, PadLeft, Columns, HumanSize,
// ColorEnabled, SetColorEnabled, ResetColorEnabled.
//
// Covers prd003-format R1.1–R1.4, R2.3, R2.6–R2.7, R3.1–R3.6.
package format

import (
	"bytes"
	"os"
	"testing"
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
		{"zero width", "abc", 0, "abc"},
		{"empty string", "", 4, "    "},
		{"unicode runes", "αβγ", 5, "αβγ  "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PadRight(tc.input, tc.width)
			if got != tc.want {
				t.Errorf("PadRight(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
			}
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
		{"zero width", "abc", 0, "abc"},
		{"empty string", "", 4, "    "},
		{"unicode runes", "αβγ", 5, "  αβγ"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PadLeft(tc.input, tc.width)
			if got != tc.want {
				t.Errorf("PadLeft(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
			}
		})
	}
}

func TestColumns_EmptyInput(t *testing.T) {
	t.Parallel()
	// R3.5: Columns returns an empty slice when given an empty entries list.
	got := Columns([]string{}, 80)
	if got != nil {
		t.Errorf("Columns([], 80) = %v, want nil", got)
	}
}

func TestColumns_SingleEntryWiderThanTermWidth(t *testing.T) {
	t.Parallel()
	// R3.4: when a single entry exceeds terminal width, place it alone on its row.
	entries := []string{"verylongfilename"}
	got := Columns(entries, 5)
	if len(got) != 1 {
		t.Fatalf("Columns() returned %d rows, want 1", len(got))
	}
	if len(got[0]) != 1 || got[0][0] != "verylongfilename" {
		t.Errorf("Columns() row 0 = %v, want [verylongfilename]", got[0])
	}
}

func TestColumns_AllEntriesWiderThanHalfTermWidth(t *testing.T) {
	t.Parallel()
	// R3.6: when no two entries fit side by side, output one entry per row.
	entries := []string{"aaaaaa", "bbbbbb", "cccccc"}
	got := Columns(entries, 10)
	if len(got) != 3 {
		t.Fatalf("Columns() returned %d rows, want 3", len(got))
	}
	for i, entry := range entries {
		if len(got[i]) != 1 || got[i][0] != entry {
			t.Errorf("row %d = %v, want [%s]", i, got[i], entry)
		}
	}
}

func TestColumns_MultipleColumnsFit(t *testing.T) {
	t.Parallel()
	// R1.1: distributes entries into the maximum number of columns that fit.
	entries := []string{"a", "b", "c", "d"}
	got := Columns(entries, 20)
	// With 4 short entries and width 20, all should fit in multiple columns.
	if len(got) == 0 {
		t.Fatal("Columns() returned empty result")
	}
	// Verify all entries are present.
	var count int
	for _, row := range got {
		count += len(row)
	}
	if count != 4 {
		t.Errorf("total entries = %d, want 4", count)
	}
}

func TestColumns_ZeroTermWidth(t *testing.T) {
	t.Parallel()
	// termWidth <= 0 falls back to single column.
	entries := []string{"a", "b", "c"}
	got := Columns(entries, 0)
	if len(got) != 3 {
		t.Fatalf("Columns(entries, 0) returned %d rows, want 3", len(got))
	}
	for i, entry := range entries {
		if len(got[i]) != 1 || got[i][0] != entry {
			t.Errorf("row %d = %v, want [%s]", i, got[i], entry)
		}
	}
}

func TestColumns_NegativeTermWidth(t *testing.T) {
	t.Parallel()
	entries := []string{"a", "b"}
	got := Columns(entries, -1)
	if len(got) != 2 {
		t.Fatalf("Columns(entries, -1) returned %d rows, want 2", len(got))
	}
}

func TestHumanSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		bytes  int64
		binary bool
		want   string
	}{
		// R3.4: zero returns "0".
		{"zero binary", 0, true, "0"},
		{"zero SI", 0, false, "0"},
		// R3.1-R3.2: binary mode (1024-based).
		{"1 byte binary", 1, true, "1"},
		{"1024 binary", 1024, true, "1K"},
		{"1536 binary", 1536, true, "1.5K"},
		{"1048576 binary", 1048576, true, "1M"},
		// R3.1-R3.2: SI mode (1000-based).
		{"1000 SI", 1000, false, "1kB"},
		{"1500 SI", 1500, false, "1.5kB"},
		{"1000000 SI", 1000000, false, "1MB"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HumanSize(tc.bytes, HumanSizeOpts{Binary: tc.binary})
			if got != tc.want {
				t.Errorf("HumanSize(%d, {Binary: %v}) = %q, want %q",
					tc.bytes, tc.binary, got, tc.want)
			}
		})
	}
}

func TestColorEnabled_NonFile(t *testing.T) {
	t.Parallel()
	// R2.3: non-*os.File writer returns false.
	var buf bytes.Buffer
	if ColorEnabled(&buf) {
		t.Error("ColorEnabled(bytes.Buffer) = true, want false")
	}
}

func TestColorEnabled_Pipe(t *testing.T) {
	t.Parallel()
	// R2.3: pipe file descriptor returns false.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	defer r.Close()
	defer w.Close()
	if ColorEnabled(w) {
		t.Error("ColorEnabled(pipe) = true, want false")
	}
}

func TestSetColorEnabled_And_Reset(t *testing.T) {
	// Not parallel: mutates package-level colorOverride.
	// R2.6: SetColorEnabled(true) forces color on.
	SetColorEnabled(true)
	if !isColorActive() {
		t.Error("isColorActive() = false after SetColorEnabled(true)")
	}

	// R2.6: SetColorEnabled(false) forces color off.
	SetColorEnabled(false)
	if isColorActive() {
		t.Error("isColorActive() = true after SetColorEnabled(false)")
	}

	// R2.7: ResetColorEnabled reverts to auto-detection.
	ResetColorEnabled()
	// After reset, color depends on whether stdout is a TTY.
	// In tests, stdout is typically a pipe, so expect false.
	// Just verify no panic and the override is cleared.
	colorMu.RLock()
	override := colorOverride
	colorMu.RUnlock()
	if override != nil {
		t.Error("colorOverride is not nil after ResetColorEnabled()")
	}
}

func TestFileTypeColor_WithOverride(t *testing.T) {
	// Not parallel: mutates package-level colorOverride.
	SetColorEnabled(true)
	defer ResetColorEnabled()

	// R2.1: directory returns blue escape.
	got := FileTypeColor(os.ModeDir)
	if got != "\033[34m" {
		t.Errorf("FileTypeColor(ModeDir) = %q, want %q", got, "\033[34m")
	}

	// R2.2: Reset returns the reset sequence.
	got = Reset()
	if got != "\033[0m" {
		t.Errorf("Reset() = %q, want %q", got, "\033[0m")
	}

	// R2.6: when disabled, returns empty.
	SetColorEnabled(false)
	got = FileTypeColor(os.ModeDir)
	if got != "" {
		t.Errorf("FileTypeColor(ModeDir) with color off = %q, want empty", got)
	}
	got = Reset()
	if got != "" {
		t.Errorf("Reset() with color off = %q, want empty", got)
	}
}
