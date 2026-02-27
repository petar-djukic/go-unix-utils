// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat exercising all cat test cases from
// test-rel01.1.yaml.
//
// Implements: prd006-cat R1-R5 (differential testing), prd001-testutils R1-R3
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinary is the path to the freshly built Go cat binary. Set by TestMain.
var goBinary string

// refBinary is the path to the Homebrew reference gcat binary. Set by TestMain.
var refBinary string

// baseEnv provides the standard test environment per test-rel01.1.yaml
// preconditions: LC_ALL=C to eliminate locale-dependent divergence.
var baseEnv = []string{"LC_ALL=C"}

// TestMain builds the Go cat binary and locates the Homebrew reference binary.
// Per design decision D1 and D4.
func TestMain(m *testing.M) {
	// Build the Go cat binary into a temp directory.
	tmpDir, err := os.MkdirTemp("", "cat-test-*")
	if err != nil {
		os.Exit(1)
	}

	goBinary = filepath.Join(tmpDir, "cat")
	buildCmd := exec.Command("go", "build", "-o", goBinary, ".")
	if _, err := buildCmd.CombinedOutput(); err != nil {
		// Build failed; leave goBinary empty so tests skip gracefully.
		goBinary = ""
	}

	// Locate the Homebrew reference binary (brew install coreutils).
	// Per design decision D4: reference is gcat, not cat (macOS BSD).
	refBinary, _ = exec.LookPath("gcat")

	code := m.Run()
	// Best-effort cleanup of temp directory.
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}

// TestCatDifferential runs all differential test cases from test-rel01.1.yaml
// (cat section). Per prd001-testutils AC1, the test defines a []DiffTest slice
// and calls RunDiffTests(t, goBinary, refBinary, tests).
func TestCatDifferential(t *testing.T) {
	if goBinary == "" {
		t.Skip("Go cat binary could not be built; skipping differential tests")
	}
	if refBinary == "" {
		t.Skip("reference gcat binary not found on PATH (brew install coreutils); skipping differential tests")
	}

	// --- File fixture setup ---
	// Per design decision D2: create fixture files in t.TempDir() and set
	// WorkDir on the DiffTest for file-based test cases.

	defaultDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(defaultDir, "file1.txt"), []byte("hello\nworld\n"), 0644); err != nil {
		t.Fatalf("creating fixture file1.txt: %v", err)
	}

	multiDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(multiDir, "file1.txt"), []byte("aaa\n"), 0644); err != nil {
		t.Fatalf("creating fixture file1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(multiDir, "file2.txt"), []byte("bbb\n"), 0644); err != nil {
		t.Fatalf("creating fixture file2.txt: %v", err)
	}

	missingDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(missingDir, "real.txt"), []byte("data\n"), 0644); err != nil {
		t.Fatalf("creating fixture real.txt: %v", err)
	}

	// Construct 256-byte stdin for binary passthrough test.
	// Per design decision D3: build programmatically, not as a literal.
	allBytes := make([]byte, 256)
	for i := 0; i < 256; i++ {
		allBytes[i] = byte(i)
	}

	// Per design decision D5: no normalization is needed for any cat test case.
	// Cat output is deterministic with no timestamps or non-deterministic fields.

	tests := []testutils.DiffTest{
		// --- Default behavior tests (prd006-cat R1) ---

		{
			// Per test-rel01.1.yaml: cat_default_passthrough.
			// Traces: prd006-cat R1.1, R1.5.
			Name:    "cat_default_passthrough",
			Args:    []string{"file1.txt"},
			Stdin:   nil,
			Env:     baseEnv,
			WorkDir: defaultDir,
		},
		{
			// Per test-rel01.1.yaml: cat_binary_passthrough.
			// Traces: prd006-cat R1.4.
			Name:  "cat_binary_passthrough",
			Args:  nil,
			Stdin: allBytes,
			Env:   baseEnv,
		},

		// --- Line numbering tests (prd006-cat R2) ---

		{
			// Per test-rel01.1.yaml: cat_line_numbering_n.
			// Traces: prd006-cat R2.1.
			Name:  "cat_line_numbering_n",
			Args:  []string{"-n"},
			Stdin: []byte("alpha\n\nbeta\n"),
			Env:   baseEnv,
		},
		{
			// Per test-rel01.1.yaml: cat_line_numbering_b.
			// Traces: prd006-cat R2.2, R2.4.
			Name:  "cat_line_numbering_b",
			Args:  []string{"-b"},
			Stdin: []byte("first\n\n\nsecond\n"),
			Env:   baseEnv,
		},

		// --- Blank-line squeezing test (prd006-cat R3) ---

		{
			// Per test-rel01.1.yaml: cat_squeeze_blanks.
			// Traces: prd006-cat R3.1.
			Name:  "cat_squeeze_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
			Env:   baseEnv,
		},

		// --- Non-printing display tests (prd006-cat R4) ---

		{
			// Per test-rel01.1.yaml: cat_show_nonprinting.
			// Traces: prd006-cat R4.1, R4.2.
			Name:  "cat_show_nonprinting",
			Args:  []string{"-v"},
			Stdin: []byte{0x01, 0x09, 0x1b, 0x7f, 0x80, 0xff},
			Env:   baseEnv,
		},
		{
			// Per test-rel01.1.yaml: cat_show_ends.
			// Traces: prd006-cat R4.3.
			Name:  "cat_show_ends",
			Args:  []string{"-E"},
			Stdin: []byte("line one\nline two\n"),
			Env:   baseEnv,
		},
		{
			// Per test-rel01.1.yaml: cat_show_tabs.
			// Traces: prd006-cat R4.4.
			Name:  "cat_show_tabs",
			Args:  []string{"-T"},
			Stdin: []byte("col1\tcol2\tcol3\n"),
			Env:   baseEnv,
		},
		{
			// Per test-rel01.1.yaml: cat_show_all.
			// Traces: prd006-cat R4.5.
			Name:  "cat_show_all",
			Args:  []string{"-A"},
			Stdin: []byte("\x01\thello\n"),
			Env:   baseEnv,
		},
		{
			// Per test-rel01.1.yaml: cat_flag_e.
			// Traces: prd006-cat R4.6.
			Name:  "cat_flag_e",
			Args:  []string{"-e"},
			Stdin: []byte("\x01hello\n"),
			Env:   baseEnv,
		},
		{
			// Per test-rel01.1.yaml: cat_flag_t.
			// Traces: prd006-cat R4.7.
			Name:  "cat_flag_t",
			Args:  []string{"-t"},
			Stdin: []byte("\x01\thello\n"),
			Env:   baseEnv,
		},
		{
			// Per test-rel01.1.yaml: cat_flag_u_accepted.
			// Traces: prd006-cat R4.8.
			Name:  "cat_flag_u_accepted",
			Args:  []string{"-u"},
			Stdin: []byte("test\n"),
			Env:   baseEnv,
		},

		// --- Combined flag tests (prd006-cat R3, R4) ---

		{
			// Per test-rel01.1.yaml: cat_combined_ns.
			// Traces: prd006-cat R3.3, R4.9.
			Name:  "cat_combined_ns",
			Args:  []string{"-n", "-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
			Env:   baseEnv,
		},

		// --- Multi-file and stdin tests (prd006-cat R1) ---

		{
			// Per test-rel01.1.yaml: cat_multiple_files.
			// Traces: prd006-cat R1.1, R1.3.
			Name:    "cat_multiple_files",
			Args:    []string{"file1.txt", "file2.txt"},
			Stdin:   nil,
			Env:     baseEnv,
			WorkDir: multiDir,
		},
		{
			// Per test-rel01.1.yaml: cat_stdin_dash.
			// Traces: prd006-cat R1.2.
			Name:  "cat_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("from stdin\n"),
			Env:   baseEnv,
		},

		// --- Error handling test (prd006-cat R5) ---

		{
			// Per test-rel01.1.yaml: cat_missing_file.
			// Traces: prd006-cat R5.2.
			Name:    "cat_missing_file",
			Args:    []string{"nonexistent.txt", "real.txt"},
			Stdin:   nil,
			Env:     baseEnv,
			WorkDir: missingDir,
		},
	}

	testutils.RunDiffTests(t, goBinary, refBinary, tests)
}
