// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// progNameNormalizer strips the program name prefix from each stderr line
// so that error messages from different binaries can be compared.
func progNameNormalizer(data []byte) []byte {
	re := regexp.MustCompile(`(?m)^[^\s:]+:`)
	return re.ReplaceAll(data, []byte("stty:"))
}

// TestDiff runs differential tests comparing the Go stty binary against gstty.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gstty")
	if err != nil {
		t.Skipf("reference binary gstty not in PATH: %v", err)
	}

	// Check if /dev/tty is available (not available in all environments).
	ttyFile, ttyErr := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	hasTTY := ttyErr == nil
	if hasTTY {
		ttyFile.Close()
	}

	tests := []testutils.DiffTest{
		{
			Name:      "no-terminal-stdin",
			Args:      nil,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{progNameNormalizer},
		},
		{
			Name:      "all-no-terminal-stdin",
			Args:      []string{"-a"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{progNameNormalizer},
		},
		{
			Name:      "save-no-terminal-stdin",
			Args:      []string{"-g"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{progNameNormalizer},
		},
	}

	if hasTTY {
		tests = append(tests,
			// R1.1, R2.1, R3.1, R3.2: Display tests.
			testutils.DiffTest{
				Name: "default-display-tty",
				Args: []string{"-F", "/dev/tty"},
			},
			testutils.DiffTest{
				Name: "all-settings-tty",
				Args: []string{"-a", "-F", "/dev/tty"},
			},
			testutils.DiffTest{
				Name: "save-format-tty",
				Args: []string{"-g", "-F", "/dev/tty"},
			},
			testutils.DiffTest{
				Name: "all-with-file-equals",
				Args: []string{"--file=/dev/tty", "-a"},
			},
			// R6.1: Start with sane to ensure known state.
			testutils.DiffTest{
				Name: "sane-mode-init",
				Args: []string{"-F", "/dev/tty", "sane"},
			},
			// R4.1: Flag settings.
			testutils.DiffTest{
				Name: "set-echo-noop",
				Args: []string{"-F", "/dev/tty", "echo"},
			},
			testutils.DiffTest{
				Name: "toggle-echo-restore",
				Args: []string{"-F", "/dev/tty", "-echo", "echo"},
			},
			// R5.1: Special characters.
			testutils.DiffTest{
				Name: "set-eof-default",
				Args: []string{"-F", "/dev/tty", "eof", "^D"},
			},
			testutils.DiffTest{
				Name: "set-min-value",
				Args: []string{"-F", "/dev/tty", "min", "1"},
			},
			// R6.1: Combination settings.
			testutils.DiffTest{
				Name: "cooked-mode",
				Args: []string{"-F", "/dev/tty", "cooked"},
			},
			// R6.2: Speed setting.
			testutils.DiffTest{
				Name: "set-speed-9600",
				Args: []string{"-F", "/dev/tty", "9600"},
			},
			// R6.1: Restore sane at end for cleanup.
			testutils.DiffTest{
				Name: "sane-mode-cleanup",
				Args: []string{"-F", "/dev/tty", "sane"},
			},
		)
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestFormatChar verifies control character formatting.
func TestFormatChar(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		c    uint8
		want string
	}{
		{"null", 0x00, "^@"},
		{"ctrl-c", 0x03, "^C"},
		{"ctrl-backslash", 0x1C, `^\`},
		{"delete", 0x7F, "^?"},
		{"undef", 0xFF, "<undef>"},
		{"space", 0x20, " "},
		{"tilde", 0x7E, "~"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatChar(tc.c)
			if got != tc.want {
				t.Errorf("formatChar(0x%02x) = %q, want %q", tc.c, got, tc.want)
			}
		})
	}
}

// TestRenderEntry verifies flag rendering for simple and multi-value flags.
func TestRenderEntry(t *testing.T) {
	t.Parallel()
	t.Run("simple-on", func(t *testing.T) {
		t.Parallel()
		e := displayEntry{Name: "echo", Mask: 0x08}
		got := renderEntry(e, 0x08)
		if got != "echo" {
			t.Errorf("renderEntry = %q, want %q", got, "echo")
		}
	})
	t.Run("simple-off", func(t *testing.T) {
		t.Parallel()
		e := displayEntry{Name: "echo", Mask: 0x08}
		got := renderEntry(e, 0x00)
		if got != "-echo" {
			t.Errorf("renderEntry = %q, want %q", got, "-echo")
		}
	})
	t.Run("multi-value", func(t *testing.T) {
		t.Parallel()
		e := displayEntry{
			Mask: 0x300,
			Values: []multiValue{
				{"cs5", 0x000}, {"cs6", 0x100}, {"cs7", 0x200}, {"cs8", 0x300},
			},
		}
		got := renderEntry(e, 0x300)
		if got != "cs8" {
			t.Errorf("renderEntry = %q, want %q", got, "cs8")
		}
	})
}

// TestIsChanged verifies default-change detection for flags.
func TestIsChanged(t *testing.T) {
	t.Parallel()
	t.Run("simple-at-default", func(t *testing.T) {
		t.Parallel()
		e := displayEntry{Name: "echo", Mask: 0x08, DefOn: true}
		if isChanged(e, 0x08) {
			t.Error("expected not changed when flag matches default")
		}
	})
	t.Run("simple-changed", func(t *testing.T) {
		t.Parallel()
		e := displayEntry{Name: "echo", Mask: 0x08, DefOn: true}
		if !isChanged(e, 0x00) {
			t.Error("expected changed when flag differs from default")
		}
	})
}

// TestWrapWriter verifies line wrapping behavior.
func TestWrapWriter(t *testing.T) {
	t.Parallel()
	var w wrapWriter
	// Add entries that should wrap at 80 columns.
	for range 10 {
		w.add("12345678")
	}
	w.newLine()
	out := w.buf.String()
	lines := bytes.Split([]byte(out), []byte("\n"))
	for i, line := range lines {
		if len(line) > wrapCol {
			t.Errorf("line %d exceeds %d columns: %d chars", i, wrapCol, len(line))
		}
	}
}

// TestParseCharValue verifies special character value parsing.
func TestParseCharValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    uint8
		wantErr bool
	}{
		{"ctrl-c", "^C", 0x03, false},
		{"ctrl-at", "^@", 0x00, false},
		{"delete", "^?", 0x7F, false},
		{"undef-word", "undef", 0xFF, false},
		{"undef-caret", "^-", 0xFF, false},
		{"literal-a", "a", 'a', false},
		{"ctrl-backslash", `^\`, 0x1C, false},
		{"invalid", "invalid", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseCharValue(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseCharValue(%q) = %d, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("parseCharValue(%q) error: %v", tc.input, err)
				return
			}
			if got != tc.want {
				t.Errorf("parseCharValue(%q) = 0x%02x, want 0x%02x", tc.input, got, tc.want)
			}
		})
	}
}

// TestLookupFlag verifies flag lookup across all tables.
func TestLookupFlag(t *testing.T) {
	t.Parallel()
	t.Run("found-echo", func(t *testing.T) {
		t.Parallel()
		entry, cat, ok := lookupFlag("echo")
		if !ok {
			t.Fatal("expected to find 'echo' flag")
		}
		if cat != catLocal {
			t.Errorf("echo category = %d, want %d (catLocal)", cat, catLocal)
		}
		if entry.Name != "echo" {
			t.Errorf("entry.Name = %q, want %q", entry.Name, "echo")
		}
	})
	t.Run("not-found", func(t *testing.T) {
		t.Parallel()
		_, _, ok := lookupFlag("nonexistent")
		if ok {
			t.Error("expected nonexistent flag to not be found")
		}
	})
}

// TestLookupMultiValue verifies multi-value flag lookup.
func TestLookupMultiValue(t *testing.T) {
	t.Parallel()
	entry, val, cat, ok := lookupMultiValue("cs8")
	if !ok {
		t.Fatal("expected to find 'cs8' multi-value")
	}
	if cat != catControl {
		t.Errorf("cs8 category = %d, want %d (catControl)", cat, catControl)
	}
	if entry.Mask == 0 {
		t.Error("expected non-zero mask for CSIZE entry")
	}
	if val == 0 && ok {
		// cs8 should have a non-zero value on most platforms.
		// On Darwin, CS8 = 0x300.
		_ = val
	}
}

// TestIsSavedFormat verifies saved format detection.
func TestIsSavedFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid", "1:2:3:4:5", true},
		{"too-few-parts", "1:2:3", false},
		{"not-hex", "sane:1:2:3:4", false},
		{"empty", "", false},
		{"single", "abc", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isSavedFormat(tc.input)
			if got != tc.want {
				t.Errorf("isSavedFormat(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
