// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/wc exercising all wc test cases from
// test-rel01.1.yaml.
//
// Implements: prd005-wc R1-R6 (differential testing), prd001-testutils R1-R3
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinary is the path to the freshly built Go wc binary. Set by TestMain.
var goBinary string

// refBinary is the path to the Homebrew reference gwc binary. Set by TestMain.
var refBinary string

// baseEnv provides the standard test environment per test-rel01.1.yaml
// preconditions: LC_ALL=C to eliminate locale-dependent divergence.
var baseEnv = []string{"LC_ALL=C"}

// TestMain builds the Go wc binary and locates the Homebrew reference binary.
// Per design decision D1 and D4.
func TestMain(m *testing.M) {
	// Build the Go wc binary into a temp directory.
	tmpDir, err := os.MkdirTemp("", "wc-test-*")
	if err != nil {
		os.Exit(1)
	}

	goBinary = filepath.Join(tmpDir, "wc")
	buildCmd := exec.Command("go", "build", "-o", goBinary, ".")
	if _, err := buildCmd.CombinedOutput(); err != nil {
		// Build failed; leave goBinary empty so tests skip gracefully.
		goBinary = ""
	}

	// Locate the Homebrew reference binary (brew install coreutils).
	// Per design decision D4: reference is gwc, not wc (macOS BSD).
	refBinary, _ = exec.LookPath("gwc")

	code := m.Run()
	// Best-effort cleanup of temp directory.
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}

// TestWcDifferential runs all differential test cases from test-rel01.1.yaml
// (wc section). Per prd001-testutils AC1, the test defines a []DiffTest slice
// and calls RunDiffTests(t, goBinary, refBinary, tests).
func TestWcDifferential(t *testing.T) {
	if goBinary == "" {
		t.Skip("Go wc binary could not be built; skipping differential tests")
	}
	if refBinary == "" {
		t.Skip("reference gwc binary not found on PATH (brew install coreutils); skipping differential tests")
	}

	// --- File fixture setup ---
	// Per design decision D3: create fixture files in t.TempDir() and set
	// WorkDir on the DiffTest for file-based test cases.

	// Fixture for wc_multi_file_total.
	// Per test-rel01.1.yaml: file1.txt contains "hello\nworld\n" (2 lines,
	// 2 words, 12 bytes), file2.txt contains "foo bar baz\n" (1 line,
	// 3 words, 12 bytes).
	multiFileDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(multiFileDir, "file1.txt"), []byte("hello\nworld\n"), 0644); err != nil {
		t.Fatalf("creating fixture file1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(multiFileDir, "file2.txt"), []byte("foo bar baz\n"), 0644); err != nil {
		t.Fatalf("creating fixture file2.txt: %v", err)
	}

	// Fixture for wc_total_only.
	// Per test-rel01.1.yaml: file1.txt contains "a\n", file2.txt contains "b\nc\n".
	totalOnlyDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(totalOnlyDir, "file1.txt"), []byte("a\n"), 0644); err != nil {
		t.Fatalf("creating fixture file1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(totalOnlyDir, "file2.txt"), []byte("b\nc\n"), 0644); err != nil {
		t.Fatalf("creating fixture file2.txt: %v", err)
	}

	// Fixture for wc_total_never.
	// Per test-rel01.1.yaml: file1.txt contains "x\n", file2.txt contains "y\n".
	totalNeverDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(totalNeverDir, "file1.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatalf("creating fixture file1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(totalNeverDir, "file2.txt"), []byte("y\n"), 0644); err != nil {
		t.Fatalf("creating fixture file2.txt: %v", err)
	}

	// Fixture for wc_missing_file.
	// Per test-rel01.1.yaml: real.txt contains "data\n", nonexistent.txt
	// does not exist.
	missingDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(missingDir, "real.txt"), []byte("data\n"), 0644); err != nil {
		t.Fatalf("creating fixture real.txt: %v", err)
	}

	// Per design decision D5: no normalization is needed for any wc test case.
	// wc output is deterministic integer counts.

	tests := []testutils.DiffTest{
		// --- Default behavior tests (prd005-wc R1) ---

		{
			// Per test-rel01.1.yaml: wc_default_three_lines.
			// Traces: prd005-wc R1.1, R1.2, R1.3.
			Name:  "wc_default_three_lines",
			Args:  nil,
			Stdin: []byte("foo\nbar baz\nqux\n"),
			Env:   baseEnv,
		},

		// --- Individual flag tests (prd005-wc R2) ---

		{
			// Per test-rel01.1.yaml: wc_lines_only.
			// Traces: prd005-wc R2.1, R2.6.
			Name:  "wc_lines_only",
			Args:  []string{"-l"},
			Stdin: []byte("one\ntwo\nthree\n"),
			Env:   baseEnv,
		},
		{
			// Per test-rel01.1.yaml: wc_words_only.
			// Traces: prd005-wc R2.2, R2.6.
			Name:  "wc_words_only",
			Args:  []string{"-w"},
			Stdin: []byte("hello world\ngoodbye  cruel   world\n"),
			Env:   baseEnv,
		},
		{
			// Per test-rel01.1.yaml: wc_bytes_only.
			// Traces: prd005-wc R2.3.
			Name:  "wc_bytes_only",
			Args:  []string{"-c"},
			Stdin: []byte("abc\n"),
			Env:   baseEnv,
		},
		{
			// Per test-rel01.1.yaml: wc_chars_lc_c.
			// Traces: prd005-wc R2.4, R5.1, R5.2.
			Name:  "wc_chars_lc_c",
			Args:  []string{"-m"},
			Stdin: []byte("hello\n"),
			Env:   baseEnv,
		},
		{
			// Per test-rel01.1.yaml: wc_max_line_length.
			// Traces: prd005-wc R2.5.
			Name:  "wc_max_line_length",
			Args:  []string{"-L"},
			Stdin: []byte("short\na much longer line here\nmed\n"),
			Env:   baseEnv,
		},

		// --- Combined flags test (prd005-wc R2.6) ---

		{
			// Per test-rel01.1.yaml: wc_combined_flags_order.
			// Traces: prd005-wc R2.6.
			Name:  "wc_combined_flags_order",
			Args:  []string{"-w", "-l", "-c"},
			Stdin: []byte("one two\nthree\n"),
			Env:   baseEnv,
		},

		// --- Multi-file and total tests (prd005-wc R1.4, R3) ---

		{
			// Per test-rel01.1.yaml: wc_multi_file_total.
			// Traces: prd005-wc R1.4, R3.1, R3.2.
			Name:    "wc_multi_file_total",
			Args:    []string{"file1.txt", "file2.txt"},
			Stdin:   nil,
			Env:     baseEnv,
			WorkDir: multiFileDir,
		},
		{
			// Per test-rel01.1.yaml: wc_total_only.
			// Traces: prd005-wc R3.3.
			Name:    "wc_total_only",
			Args:    []string{"--total=only", "file1.txt", "file2.txt"},
			Stdin:   nil,
			Env:     baseEnv,
			WorkDir: totalOnlyDir,
		},
		{
			// Per test-rel01.1.yaml: wc_total_never.
			// Traces: prd005-wc R3.3.
			Name:    "wc_total_never",
			Args:    []string{"--total=never", "file1.txt", "file2.txt"},
			Stdin:   nil,
			Env:     baseEnv,
			WorkDir: totalNeverDir,
		},

		// --- Stdin and special input tests (prd005-wc R4) ---

		{
			// Per test-rel01.1.yaml: wc_stdin_dash.
			// Traces: prd005-wc R4.1.
			Name:  "wc_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("stdin content\n"),
			Env:   baseEnv,
		},
		{
			// Per test-rel01.1.yaml: wc_empty_stdin.
			// Traces: prd005-wc R4.3.
			Name:  "wc_empty_stdin",
			Args:  nil,
			Stdin: []byte{},
			Env:   baseEnv,
		},

		// --- Error handling test (prd005-wc R6) ---

		{
			// Per test-rel01.1.yaml: wc_missing_file.
			// Traces: prd005-wc R6.2.
			Name:    "wc_missing_file",
			Args:    []string{"nonexistent.txt", "real.txt"},
			Stdin:   nil,
			Env:     baseEnv,
			WorkDir: missingDir,
		},
	}

	testutils.RunDiffTests(t, goBinary, refBinary, tests)
}
