// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// difftest_test.go contains unit tests for the differential testing harness:
// RunDiffTests, TimestampNormalizer, ComposeNormalizers, buildEnv, and
// truncateStdin.
//
// Tests: prd001-testutils R2, R3, R4, R5.
package testutils

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunDiffTests_PassCase verifies that RunDiffTests reports no divergence
// when both binaries produce identical output (prd001-testutils R2, R3).
func TestRunDiffTests_PassCase(t *testing.T) {
	tests := []DiffTest{
		{
			Name:     "echo_hello",
			Args:     []string{"hello"},
			ExitCode: 0,
		},
		{
			Name:     "echo_multiple_args",
			Args:     []string{"hello", "world"},
			ExitCode: 0,
		},
		{
			Name:     "echo_no_args",
			Args:     nil,
			ExitCode: 0,
		},
	}

	// Using /bin/echo as both binaries guarantees identical output.
	RunDiffTests(t, "/bin/echo", "/bin/echo", tests)
}

// TestRunDiffTests_WithStdin verifies that stdin is piped identically to both
// binaries (prd001-testutils R2.1, R2.2).
func TestRunDiffTests_WithStdin(t *testing.T) {
	tests := []DiffTest{
		{
			Name:     "cat_stdin",
			Args:     []string{"-c", "cat"},
			Stdin:    []byte("hello from stdin\n"),
			ExitCode: 0,
		},
	}

	// /bin/sh -c cat reads stdin and writes it to stdout.
	RunDiffTests(t, "/bin/sh", "/bin/sh", tests)
}

// TestTimestampNormalizer verifies that all three timestamp formats are replaced
// with the placeholder string (prd001-testutils R4.2).
func TestTimestampNormalizer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "month_day_time",
			input:    "Feb 19 12:34:56 some event",
			expected: "<TIMESTAMP> some event",
		},
		{
			name:     "iso_datetime",
			input:    "2026-02-19 12:34:56 some event",
			expected: "<TIMESTAMP> some event",
		},
		{
			name:     "time_only",
			input:    "12:34:56 some event",
			expected: "<TIMESTAMP> some event",
		},
		{
			name:     "no_timestamp",
			input:    "no timestamp here",
			expected: "no timestamp here",
		},
		{
			name:     "multiple_timestamps",
			input:    "start 09:00:00 end 17:30:00",
			expected: "start <TIMESTAMP> end <TIMESTAMP>",
		},
		{
			name:     "empty_input",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(TimestampNormalizer([]byte(tt.input)))
			if got != tt.expected {
				t.Errorf("TimestampNormalizer(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestComposeNormalizers verifies that composed normalizers are applied in
// left-to-right order (prd001-testutils R4.4).
func TestComposeNormalizers(t *testing.T) {
	// First normalizer: replace "A" with "B".
	replaceAtoB := func(data []byte) []byte {
		return bytes.ReplaceAll(data, []byte("A"), []byte("B"))
	}
	// Second normalizer: replace "B" with "C".
	replaceBtoC := func(data []byte) []byte {
		return bytes.ReplaceAll(data, []byte("B"), []byte("C"))
	}

	t.Run("left_to_right_order", func(t *testing.T) {
		// A -> B (first), then B -> C (second). Result: "C".
		composed := ComposeNormalizers(replaceAtoB, replaceBtoC)
		got := string(composed([]byte("A")))
		if got != "C" {
			t.Errorf("ComposeNormalizers(A->B, B->C)(\"A\") = %q, want %q", got, "C")
		}
	})

	t.Run("reverse_order_differs", func(t *testing.T) {
		// B -> C (first), then A -> B (second). "A" is unaffected by first, then A -> B.
		composed := ComposeNormalizers(replaceBtoC, replaceAtoB)
		got := string(composed([]byte("A")))
		if got != "B" {
			t.Errorf("ComposeNormalizers(B->C, A->B)(\"A\") = %q, want %q", got, "B")
		}
	})

	t.Run("single_normalizer", func(t *testing.T) {
		composed := ComposeNormalizers(replaceAtoB)
		got := string(composed([]byte("A")))
		if got != "B" {
			t.Errorf("ComposeNormalizers(A->B)(\"A\") = %q, want %q", got, "B")
		}
	})

	t.Run("no_normalizers", func(t *testing.T) {
		composed := ComposeNormalizers()
		got := string(composed([]byte("unchanged")))
		if got != "unchanged" {
			t.Errorf("ComposeNormalizers()(\"unchanged\") = %q, want %q", got, "unchanged")
		}
	})
}

// TestBuildEnv verifies that buildEnv sets LC_ALL=C by default and correctly
// merges DiffTest.Env overrides (prd001-testutils R2.6).
func TestBuildEnv(t *testing.T) {
	t.Run("default_lc_all", func(t *testing.T) {
		env := buildEnv(nil)
		val := envValue(env, "LC_ALL")
		if val != "C" {
			t.Errorf("buildEnv(nil) LC_ALL = %q, want %q", val, "C")
		}
	})

	t.Run("override_lc_all", func(t *testing.T) {
		env := buildEnv([]string{"LC_ALL=en_US.UTF-8"})
		val := envValue(env, "LC_ALL")
		if val != "en_US.UTF-8" {
			t.Errorf("buildEnv with LC_ALL override = %q, want %q", val, "en_US.UTF-8")
		}
	})

	t.Run("add_new_var", func(t *testing.T) {
		env := buildEnv([]string{"MY_TEST_VAR=hello"})
		val := envValue(env, "MY_TEST_VAR")
		if val != "hello" {
			t.Errorf("buildEnv with new var MY_TEST_VAR = %q, want %q", val, "hello")
		}
		// LC_ALL=C should still be set.
		lcVal := envValue(env, "LC_ALL")
		if lcVal != "C" {
			t.Errorf("buildEnv LC_ALL after adding new var = %q, want %q", lcVal, "C")
		}
	})

	t.Run("later_override_wins", func(t *testing.T) {
		env := buildEnv([]string{"MY_VAR=first", "MY_VAR=second"})
		val := envValue(env, "MY_VAR")
		if val != "second" {
			t.Errorf("buildEnv with duplicate key MY_VAR = %q, want %q", val, "second")
		}
	})
}

// TestTruncateStdin verifies the display-safe representation of stdin content
// (prd001-testutils R3.5).
func TestTruncateStdin(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "nil_input",
			input:    nil,
			expected: "<nil>",
		},
		{
			name:     "empty_input",
			input:    []byte{},
			expected: "",
		},
		{
			name:     "short_input",
			input:    []byte("hello world"),
			expected: "hello world",
		},
		{
			name:     "exactly_256_bytes",
			input:    []byte(strings.Repeat("x", 256)),
			expected: strings.Repeat("x", 256),
		},
		{
			name:     "exceeds_256_bytes",
			input:    []byte(strings.Repeat("y", 300)),
			expected: strings.Repeat("y", 256) + "... (300 bytes total)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStdin(tt.input)
			if got != tt.expected {
				t.Errorf("truncateStdin(%d bytes) = %q, want %q",
					len(tt.input), got, tt.expected)
			}
		})
	}
}

// TestCheckExpectedFiles verifies file-state comparison after binary execution
// (prd001-testutils R5).
func TestCheckExpectedFiles(t *testing.T) {
	t.Run("matching_file", func(t *testing.T) {
		dir := t.TempDir()
		// Write a file that matches the expectation.
		writeTestFile(t, dir, "output.txt", []byte("expected content"))

		tc := DiffTest{
			Name: "matching",
			ExpectedFiles: map[string][]byte{
				"output.txt": []byte("expected content"),
			},
		}
		// Should not fail.
		checkExpectedFiles(t, tc, dir)
	})

	t.Run("nil_expected_files", func(t *testing.T) {
		dir := t.TempDir()
		tc := DiffTest{
			Name:          "no_files",
			ExpectedFiles: nil,
		}
		// Should not fail — nil map means no file checks.
		checkExpectedFiles(t, tc, dir)
	})
}

// envValue finds the value for a given key in an environment slice.
func envValue(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):]
		}
	}
	return ""
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name string, content []byte) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
}
