// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/head against ghead (GNU coreutils).
// Implements prd018-head R1.1-R1.5, R2.1-R2.3, R3.1-R3.5, R4.1-R4.4 test coverage.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghead")
	if err != nil {
		t.Skipf("reference binary ghead not in PATH: %v", err)
	}

	// Create test fixtures.
	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "twenty.txt", generateLines(1, 20))
	writeTestFile(t, tmpDir, "five.txt", generateLines(1, 5))
	writeTestFile(t, tmpDir, "no-trailing-newline.txt", "line1\nline2\nline3")
	writeTestFile(t, tmpDir, "empty.txt", "")
	writeTestFile(t, tmpDir, "single-line.txt", "only\n")

	tests := []testutils.DiffTest{
		// R1.1: default 10 lines from stdin.
		{
			Name:  "R1.1_default_10_lines_stdin",
			Stdin: []byte(generateLines(1, 20)),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: default 10 lines from file.
		{
			Name:    "R1.1_default_10_lines_file",
			Args:    []string{filepath.Join(tmpDir, "twenty.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// R1.1: input shorter than 10 lines.
		{
			Name:    "R1.1_fewer_than_10_lines",
			Args:    []string{filepath.Join(tmpDir, "five.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// R1.4: stdin when no arguments.
		{
			Name:  "R1.4_stdin_no_args",
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: "-" means stdin.
		{
			Name:  "R1.4_dash_is_stdin",
			Args:  []string{"-"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: no trailing newline — last line still counted.
		{
			Name:    "R1.5_no_trailing_newline",
			Args:    []string{filepath.Join(tmpDir, "no-trailing-newline.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// R1.5: empty file produces no output.
		{
			Name:    "R1.5_empty_file",
			Args:    []string{filepath.Join(tmpDir, "empty.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},

		// R1.2: -n NUM overrides line count.
		{
			Name:  "R1.2_n_flag_explicit",
			Args:  []string{"-n", "3"},
			Stdin: []byte(generateLines(1, 20)),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: --lines=NUM long form.
		{
			Name:  "R1.2_lines_long_form",
			Args:  []string{"--lines=5"},
			Stdin: []byte(generateLines(1, 20)),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: -n with value joined (e.g., -n3).
		{
			Name:  "R1.2_n_joined",
			Args:  []string{"-n3"},
			Stdin: []byte(generateLines(1, 20)),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: -NUM shorthand.
		{
			Name:  "R1.2_NUM_shorthand",
			Args:  []string{"-5"},
			Stdin: []byte(generateLines(1, 20)),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: -n 0 prints nothing.
		{
			Name:  "R1.2_n_zero",
			Args:  []string{"-n", "0"},
			Stdin: []byte(generateLines(1, 5)),
			Env:   []string{"LC_ALL=C"},
		},

		// R1.3: negative count — all but last N lines.
		{
			Name:  "R1.3_negative_n",
			Args:  []string{"-n", "-5"},
			Stdin: []byte(generateLines(1, 20)),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: negative count larger than input — prints nothing.
		{
			Name:  "R1.3_negative_n_larger_than_input",
			Args:  []string{"-n", "-100"},
			Stdin: []byte(generateLines(1, 5)),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: negative count of 0 — prints all.
		{
			Name:  "R1.3_negative_n_zero",
			Args:  []string{"-n", "-0"},
			Stdin: []byte(generateLines(1, 5)),
			Env:   []string{"LC_ALL=C"},
		},

		// R1.2: multi-file with headers.
		{
			Name: "R1.2_multifile_headers",
			Args: []string{
				filepath.Join(tmpDir, "five.txt"),
				filepath.Join(tmpDir, "single-line.txt"),
			},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// R1.2: single file no header.
		{
			Name:    "R1.2_single_file_no_header",
			Args:    []string{filepath.Join(tmpDir, "five.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},

		// R2.1: -c NUM byte count mode.
		{
			Name:  "R2.1_c_flag",
			Args:  []string{"-c", "5"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: --bytes=NUM long form.
		{
			Name:  "R2.1_bytes_long_form",
			Args:  []string{"--bytes=3"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -c 0 prints nothing.
		{
			Name:  "R2.1_c_zero",
			Args:  []string{"-c", "0"},
			Stdin: []byte("abcdef"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -c larger than input — prints everything.
		{
			Name:  "R2.1_c_larger_than_input",
			Args:  []string{"-c", "100"},
			Stdin: []byte("short"),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.2: negative byte count — all but last N bytes.
		{
			Name:  "R2.2_negative_c",
			Args:  []string{"-c", "-3"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: negative byte count larger than input.
		{
			Name:  "R2.2_negative_c_larger",
			Args:  []string{"-c", "-100"},
			Stdin: []byte("short"),
			Env:   []string{"LC_ALL=C"},
		},

		// R2.3: multiplier suffix b (512 bytes).
		{
			Name:  "R2.3_suffix_b",
			Args:  []string{"-c", "1b"},
			Stdin: []byte(strings.Repeat("x", 1024)),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: multiplier suffix K (1024 bytes).
		{
			Name:  "R2.3_suffix_K",
			Args:  []string{"-c", "1K"},
			Stdin: []byte(strings.Repeat("y", 2048)),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: multiplier suffix KiB (1024 bytes).
		{
			Name:  "R2.3_suffix_KiB",
			Args:  []string{"-c", "1KiB"},
			Stdin: []byte(strings.Repeat("z", 2048)),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -c and -n mutually exclusive, last one wins.
		{
			Name:  "R2.1_last_flag_wins_c_after_n",
			Args:  []string{"-n", "1", "-c", "5"},
			Stdin: []byte("abcdefghij\nklmnop\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -n after -c, last one wins.
		{
			Name:  "R2.1_last_flag_wins_n_after_c",
			Args:  []string{"-c", "5", "-n", "1"},
			Stdin: []byte("abcdefghij\nklmnop\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R4.2: non-existent file — exit 1.
		{
			Name:      "R4.2_nonexistent_file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.2: non-existent mixed with existing — outputs good file, exit 1.
		{
			Name: "R4.2_nonexistent_mixed",
			Args: []string{
				filepath.Join(tmpDir, "five.txt"),
				filepath.Join(tmpDir, "nonexistent.txt"),
				filepath.Join(tmpDir, "single-line.txt"),
			},
			WorkDir:   tmpDir,
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},

		// R3.1-R3.4/AC1: negative line count on a file.
		{
			Name:    "R3.1_negative_n_file",
			Args:    []string{"-n", "-5", filepath.Join(tmpDir, "twenty.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// R3.4/AC2: negative byte count on a file.
		{
			Name:    "R3.4_negative_c_file",
			Args:    []string{"-c", "-5", filepath.Join(tmpDir, "twenty.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},

		// R1.3: -n with negative on multi-file.
		{
			Name: "R1.3_negative_n_multifile",
			Args: []string{
				"-n", "-2",
				filepath.Join(tmpDir, "five.txt"),
				filepath.Join(tmpDir, "single-line.txt"),
			},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},

		// R2.1: -c on multi-file.
		{
			Name: "R2.1_c_multifile",
			Args: []string{
				"-c", "3",
				filepath.Join(tmpDir, "five.txt"),
				filepath.Join(tmpDir, "single-line.txt"),
			},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffQuietVerbose tests -q and -v header control.
func TestDiffQuietVerbose(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghead")
	if err != nil {
		t.Skipf("reference binary ghead not in PATH: %v", err)
	}

	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "a.txt", "aaa\n")
	writeTestFile(t, tmpDir, "b.txt", "bbb\n")

	tests := []testutils.DiffTest{
		// R3.3: -q suppresses headers on multi-file.
		{
			Name: "R3.3_quiet_multifile",
			Args: []string{
				"-q",
				filepath.Join(tmpDir, "a.txt"),
				filepath.Join(tmpDir, "b.txt"),
			},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// R3.4: -v forces header on single file.
		{
			Name:    "R3.4_verbose_single_file",
			Args:    []string{"-v", filepath.Join(tmpDir, "a.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// generateLines produces lines "1\n2\n...\nn\n".
func generateLines(from, to int) string {
	var b strings.Builder
	for i := from; i <= to; i++ {
		fmt.Fprintf(&b, "%d\n", i)
	}
	return b.String()
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", name, err)
	}
}

// normalizeProgramName normalizes error messages for differential comparison.
// GNU head reports errors as "ghead:" while our binary uses "head:".
func normalizeProgramName(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("ghead: "), []byte("head: "))
	return bytes.ToLower(b)
}
