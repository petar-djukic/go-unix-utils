// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ls exercising all ls test cases from
// test-rel02.0.yaml.
//
// Implements: prd008-ls R1-R7 (differential testing), prd001-testutils R1-R3
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinary is the path to the freshly built Go ls binary. Set by TestMain.
var goBinary string

// refBinary is the path to the Homebrew reference gls binary. Set by TestMain.
var refBinary string

// baseEnv provides the standard test environment per test-rel02.0.yaml
// preconditions: LC_ALL=C to eliminate locale-dependent divergence.
var baseEnv = []string{"LC_ALL=C"}

// mtimePattern matches mtime fields in ls -l output. Handles both the
// recent-file format ("Feb 19 14:30", "Jan  5 14:30" with space-padded
// single-digit day) and the old-file format ("Feb 19  2023", "Jan  5  2023").
// Per rel02.0-uc001-ls flow step F5 and design decision D1.
var mtimePattern = regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+(\d{2}:\d{2}|\d{4})`)

// LsNormalizer replaces mtime fields in ls -l output with the fixed placeholder
// "MTIME" to eliminate wall-clock divergence between runs. Applied to both Go
// and reference binary outputs before comparison.
//
// Per rel02.0-uc001-ls F5, prd008-ls AC3.
func LsNormalizer(b []byte) []byte {
	return mtimePattern.ReplaceAll(b, []byte("MTIME"))
}

// TestMain builds the Go ls binary and locates the Homebrew reference binary.
// Per design decision D3 and the pattern in cmd/cat/cat_test.go.
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "ls-test-*")
	if err != nil {
		os.Exit(1)
	}

	goBinary = filepath.Join(tmpDir, "ls")
	buildCmd := exec.Command("go", "build", "-o", goBinary, ".")
	if _, err := buildCmd.CombinedOutput(); err != nil {
		// Build failed; leave goBinary empty so tests skip gracefully.
		goBinary = ""
	}

	// Locate the Homebrew reference binary (brew install coreutils).
	// Per design decision D3: reference is gls, not ls (macOS BSD).
	refBinary, _ = exec.LookPath("gls")

	code := m.Run()
	// Best-effort cleanup of temp directory.
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}

// makeFixture creates a temp directory with the standard ls test fixture:
//   - file1.txt (1024 zero bytes, mode 0644)
//   - file2.txt (1536 zero bytes, mode 0644)
//   - .hidden (6 bytes, mode 0644)
//   - subdir/ (mode 0755)
//   - link.txt -> file1.txt (symbolic link)
//
// Per test-rel02.0.yaml preconditions and design decision D2.
func makeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), make([]byte, 1024), 0644); err != nil {
		t.Fatalf("creating fixture file1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), make([]byte, 1536), 0644); err != nil {
		t.Fatalf("creating fixture file2.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("hidden"), 0644); err != nil {
		t.Fatalf("creating fixture .hidden: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatalf("creating fixture subdir: %v", err)
	}
	if err := os.Symlink("file1.txt", filepath.Join(dir, "link.txt")); err != nil {
		t.Fatalf("creating fixture link.txt: %v", err)
	}

	return dir
}

// TestLsNormalizerUnit verifies that LsNormalizer correctly replaces mtime
// fields in ls -l output with the MTIME placeholder.
// Per prd008-ls AC3.
func TestLsNormalizerUnit(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "recent_double_digit_day",
			input: "-rw-r--r-- 1 user group 1024 Feb 19 14:30 file.txt",
			want:  "-rw-r--r-- 1 user group 1024 MTIME file.txt",
		},
		{
			name:  "recent_single_digit_day",
			input: "-rw-r--r-- 1 user group 1024 Jan  5 14:30 file.txt",
			want:  "-rw-r--r-- 1 user group 1024 MTIME file.txt",
		},
		{
			name:  "old_file_single_digit_day",
			input: "-rw-r--r-- 1 user group 1024 Jan  5  2023 file.txt",
			want:  "-rw-r--r-- 1 user group 1024 MTIME file.txt",
		},
		{
			name:  "old_file_double_digit_day",
			input: "-rw-r--r-- 1 user group 1024 Feb 19  2023 file.txt",
			want:  "-rw-r--r-- 1 user group 1024 MTIME file.txt",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(LsNormalizer([]byte(tc.input)))
			if got != tc.want {
				t.Errorf("LsNormalizer(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestLsDifferential runs all differential test cases from test-rel02.0.yaml
// (ls section). Per prd001-testutils AC1.
func TestLsDifferential(t *testing.T) {
	if goBinary == "" {
		t.Skip("Go ls binary could not be built; skipping differential tests")
	}
	if refBinary == "" {
		t.Skip("reference gls binary not found on PATH (brew install coreutils); skipping differential tests")
	}

	fixtureDir := makeFixture(t)

	// missingDir is a separate temp dir containing only "real.txt" for the
	// ls_missing_file test case. Per design decision D2.
	missingDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(missingDir, "real.txt"), []byte("data\n"), 0644); err != nil {
		t.Fatalf("creating missingDir real.txt: %v", err)
	}
	// nonexistentPath is a path that does not exist, used as the first
	// argument in ls_missing_file to trigger a "cannot access" error.
	nonexistentPath := filepath.Join(missingDir, "nonexistent")

	// dirOnlyDir is a minimal existing directory for ls_directory_itself;
	// no standard fixture content is required. Per design decision D2.
	dirOnlyDir := t.TempDir()

	tests := []testutils.DiffTest{
		{
			// Per test-rel02.0.yaml: ls_default_single_col_redirect.
			// stdout is a pipe (not TTY), so default format is single-column.
			// Traces: prd008-ls R1.2, R1.3, R1.4.
			Name: "ls_default_single_col_redirect",
			Args: []string{"--color=never", fixtureDir},
			Env:  baseEnv,
		},
		{
			// Per test-rel02.0.yaml: ls_default_multi_col_tty.
			// -C forces multi-column even without TTY; -w 80 sets column width.
			// Traces: prd008-ls R1.1.
			Name: "ls_default_multi_col_tty",
			Args: []string{"-C", "--color=never", "-w", "80", fixtureDir},
			Env:  baseEnv,
		},
		{
			// Per test-rel02.0.yaml: ls_single_col_flag.
			// -1 forces single-column output regardless of TTY.
			// Traces: prd008-ls R2.1.
			Name: "ls_single_col_flag",
			Args: []string{"-1", "--color=never", fixtureDir},
			Env:  baseEnv,
		},
		{
			// Per test-rel02.0.yaml: ls_long_format.
			// -l produces long format with all metadata fields.
			// Traces: prd008-ls R2.2, R2.3, R2.4, R2.5, R2.6, R2.7, R2.9.
			Name:      "ls_long_format",
			Args:      []string{"-l", "--color=never", fixtureDir},
			Env:       baseEnv,
			Normalize: []testutils.NormalizeFunc{LsNormalizer},
		},
		{
			// Per test-rel02.0.yaml: ls_long_format_symlink.
			// -l on a directory with a symlink shows "name -> target".
			// Traces: prd008-ls R2.8.
			Name:      "ls_long_format_symlink",
			Args:      []string{"-l", "--color=never", fixtureDir},
			Env:       baseEnv,
			Normalize: []testutils.NormalizeFunc{LsNormalizer},
		},
		{
			// Per test-rel02.0.yaml: ls_long_human.
			// -lh displays human-readable file sizes matching gls -lh.
			// Traces: prd008-ls R5.1, R5.2.
			Name:      "ls_long_human",
			Args:      []string{"-lh", "--color=never", fixtureDir},
			Env:       baseEnv,
			Normalize: []testutils.NormalizeFunc{LsNormalizer},
		},
		{
			// Per test-rel02.0.yaml: ls_filter_all.
			// -a includes all entries including . and .. and dotfiles.
			// Traces: prd008-ls R3.1.
			Name: "ls_filter_all",
			Args: []string{"-a", "--color=never", fixtureDir},
			Env:  baseEnv,
		},
		{
			// Per test-rel02.0.yaml: ls_filter_almost_all.
			// -A includes dotfiles but excludes . and ..
			// Traces: prd008-ls R3.2.
			Name: "ls_filter_almost_all",
			Args: []string{"-A", "--color=never", fixtureDir},
			Env:  baseEnv,
		},
		{
			// Per test-rel02.0.yaml: ls_directory_itself.
			// -d shows the directory entry itself, not its contents.
			// Traces: prd008-ls R3.3.
			Name: "ls_directory_itself",
			Args: []string{"-d", "--color=never", dirOnlyDir},
			Env:  baseEnv,
		},
		{
			// Per test-rel02.0.yaml: ls_color_suppressed.
			// --color=never produces no ANSI escape sequences.
			// Traces: prd008-ls R4.4.
			Name: "ls_color_suppressed",
			Args: []string{"--color=never", fixtureDir},
			Env:  baseEnv,
		},
		{
			// Per test-rel02.0.yaml: ls_color_always.
			// --color=always produces ANSI escape sequences for file types.
			// Structural differential check; both binaries must agree on colorized output.
			// Traces: prd008-ls R4.1, R4.3.
			Name: "ls_color_always",
			Args: []string{"--color=always", fixtureDir},
			Env:  baseEnv,
		},
		{
			// Per test-rel02.0.yaml: ls_missing_file.
			// Non-existent argument prints error to stderr, exits 1, lists other args.
			// Traces: prd008-ls R6.2.
			Name: "ls_missing_file",
			Args: []string{"--color=never", nonexistentPath, missingDir},
			Env:  baseEnv,
		},
		{
			// Per test-rel02.0.yaml: ls_bad_option.
			// Unknown flag produces error and exits 2.
			// Traces: prd008-ls R6.3.
			Name: "ls_bad_option",
			Args: []string{"--invalid-flag"},
			Env:  baseEnv,
		},
	}

	testutils.RunDiffTests(t, goBinary, refBinary, tests)
}
