// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/head against GNU ghead.
// Covers prd018-head R4.1-R4.4 (exit codes and differential testing).
// TODO: prd018 R4.4 -z/--zero-terminated tests skipped — -z is a non_goal per E6.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU ghead and Go head.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?head|ghead`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	noSuch := regexp.MustCompile(`(?i)no such file or directory`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("head"))
		b = tryHelp.ReplaceAll(b, nil)
		b = noSuch.ReplaceAll(b, []byte("No such file or directory"))
		return b
	}
}

// generateLines builds input with numbered lines for testing.
func generateLines(n int) []byte {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString(strings.Repeat("x", i))
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// writeTestFile creates a file with content in dir and returns its path.
func writeTestFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write test file %s: %v", p, err)
	}
	return p
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghead")
	if err != nil {
		t.Skipf("reference binary ghead not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	// Prepare test data.
	input20 := generateLines(20)
	input5 := generateLines(5)
	inputNoTrail := []byte("abc\ndef\nghi")
	inputBytes := []byte("abcdefghijklmnopqrstuvwxyz")

	tests := []testutils.DiffTest{
		// R4.1/R4.4: default 10 lines from stdin.
		{
			Name:  "default_10_lines",
			Stdin: input20,
		},
		// R4.4: explicit -n count.
		{
			Name:  "n_5_lines",
			Args:  []string{"-n", "5"},
			Stdin: input20,
		},
		// R4.4: --lines= long form.
		{
			Name:  "lines_long_form",
			Args:  []string{"--lines=3"},
			Stdin: input20,
		},
		// R4.4: -n 0 prints nothing.
		{
			Name:  "n_zero_lines",
			Args:  []string{"-n", "0"},
			Stdin: input20,
		},
		// R4.4: count exceeds input — prints all.
		{
			Name:  "n_exceeds_input",
			Args:  []string{"-n", "100"},
			Stdin: input5,
		},
		// R4.4: negative line count — all but last 5.
		{
			Name:  "negative_n_5",
			Args:  []string{"-n", "-5"},
			Stdin: input20,
		},
		// R4.4: negative count exceeds input — prints nothing.
		{
			Name:  "negative_n_exceeds",
			Args:  []string{"-n", "-100"},
			Stdin: input5,
		},
		// R4.4: input without trailing newline.
		{
			Name:  "no_trailing_newline",
			Args:  []string{"-n", "2"},
			Stdin: inputNoTrail,
		},
		// R4.4: -c byte count.
		{
			Name:  "c_5_bytes",
			Args:  []string{"-c", "5"},
			Stdin: inputBytes,
		},
		// R4.4: --bytes= long form.
		{
			Name:  "bytes_long_form",
			Args:  []string{"--bytes=10"},
			Stdin: inputBytes,
		},
		// R4.4: -c 0 prints nothing.
		{
			Name:  "c_zero_bytes",
			Args:  []string{"-c", "0"},
			Stdin: inputBytes,
		},
		// R4.4: byte count exceeds input.
		{
			Name:  "c_exceeds_input",
			Args:  []string{"-c", "1000"},
			Stdin: inputBytes,
		},
		// R4.4: negative byte count — all but last 10 bytes.
		{
			Name:  "negative_c_10",
			Args:  []string{"-c", "-10"},
			Stdin: inputBytes,
		},
		// R4.4: negative byte count exceeds input.
		{
			Name:  "negative_c_exceeds",
			Args:  []string{"-c", "-1000"},
			Stdin: inputBytes,
		},
		// R4.4: empty stdin.
		{
			Name:  "empty_stdin",
			Stdin: []byte{},
		},
		// R4.4: stdin via - argument.
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: input5,
		},
		// R4.4: -n combined with -n flag (last wins).
		{
			Name:  "n_last_wins",
			Args:  []string{"-n", "3", "-n", "1"},
			Stdin: input20,
		},
		// R4.4: -c and -n combined (last wins).
		{
			Name:  "c_overrides_n",
			Args:  []string{"-n", "3", "-c", "5"},
			Stdin: input20,
		},
		// R4.4: -n overrides -c (last wins).
		{
			Name:  "n_overrides_c",
			Args:  []string{"-c", "5", "-n", "3"},
			Stdin: input20,
		},
		// R4.2/R4.4: non-existent file — exit 1.
		{
			Name:      "nonexistent_file",
			Args:      []string{"/nonexistent-path/no-such-file.txt"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffMultiFile tests multi-file headers, quiet, and verbose modes.
// Uses temp files so both binaries read the same content.
func TestDiffMultiFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghead")
	if err != nil {
		t.Skipf("reference binary ghead not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	dir := t.TempDir()
	content1 := []byte("file1 line1\nfile1 line2\nfile1 line3\n")
	content2 := []byte("file2 line1\nfile2 line2\n")
	f1 := writeTestFile(t, dir, "file1.txt", content1)
	f2 := writeTestFile(t, dir, "file2.txt", content2)

	tests := []testutils.DiffTest{
		// R3.2/R4.4: single file — no header.
		{
			Name: "single_file_no_header",
			Args: []string{f1},
		},
		// R3.1/R4.4: two files — headers printed.
		{
			Name: "two_files_with_headers",
			Args: []string{f1, f2},
		},
		// R3.3/R4.4: -q suppresses headers.
		{
			Name: "quiet_mode",
			Args: []string{"-q", f1, f2},
		},
		// R3.3/R4.4: --quiet long form.
		{
			Name: "quiet_long_form",
			Args: []string{"--quiet", f1, f2},
		},
		// R3.3/R4.4: --silent synonym.
		{
			Name: "silent_synonym",
			Args: []string{"--silent", f1, f2},
		},
		// R3.4/R4.4: -v forces header for single file.
		{
			Name: "verbose_single_file",
			Args: []string{"-v", f1},
		},
		// R3.4/R4.4: --verbose long form.
		{
			Name: "verbose_long_form",
			Args: []string{"--verbose", f1},
		},
		// R4.4: -n with multi-file.
		{
			Name: "n_2_multi_file",
			Args: []string{"-n", "2", f1, f2},
		},
		// R4.4: -c with multi-file.
		{
			Name: "c_5_multi_file",
			Args: []string{"-c", "5", f1, f2},
		},
		// R4.2/R4.4: missing file among valid files — partial error.
		{
			Name:      "missing_file_with_valid",
			Args:      []string{f1, "/nonexistent/missing.txt", f2},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.4: negative count with multi-file.
		{
			Name: "negative_n_multi_file",
			Args: []string{"-n", "-1", f1, f2},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
