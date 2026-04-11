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
