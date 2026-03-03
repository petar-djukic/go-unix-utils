// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cat differential tests verify output parity between the Go cat binary and
// the GNU reference binary gcat (Homebrew coreutils). All tests run with
// LC_ALL=C to eliminate locale-dependent divergence. No normalization is applied
// because cat output is deterministic.
//
// Implements: prd006-cat R1-R5
// Architecture: docs/ARCHITECTURE.yaml (cmd/ component, DD2, DD4, DD6)
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var (
	goBin  string
	refBin string
)

func TestMain(m *testing.M) {
	ref, err := exec.LookPath("gcat")
	if err == nil {
		refBin = ref
	}

	tmpDir, err := os.MkdirTemp("", "cat-test-*")
	if err != nil {
		os.Stderr.WriteString("failed to create temp dir: " + err.Error() + "\n")
		os.Exit(1)
	}

	goBin = filepath.Join(tmpDir, "cat")
	build := exec.Command("go", "build", "-o", goBin, ".")
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		os.RemoveAll(tmpDir) // best-effort cleanup
		os.Stderr.WriteString("go build failed: " + string(out) + "\n")
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir) // best-effort cleanup
	os.Exit(code)
}

// skipIfMissing skips the current test when gcat is not available on PATH.
func skipIfMissing(t *testing.T) {
	t.Helper()
	if refBin == "" {
		t.Skip("gcat not found in PATH")
	}
}

// writeFile creates a file in dir with the given content.
func writeFile(t *testing.T, dir, name string, content []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), content, 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// progNameNormalizer replaces "gcat:" with "cat:" in output so error messages
// from the GNU reference binary (installed as gcat) match the Go binary's
// "cat:" prefix.
func progNameNormalizer(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gcat:"), []byte("cat:"))
}

// errPresenceNormalizer replaces any non-empty output with a fixed marker.
// Used for test cases where stderr format differs between implementations but
// both must produce non-empty error output.
func errPresenceNormalizer(b []byte) []byte {
	if len(b) > 0 {
		return []byte("OUTPUT\n")
	}
	return b
}

// TestCatDefaultBehavior tests basic file concatenation, stdin reading, and the
// "-" operand per prd006-cat R1.
// (prd006-cat R1.1, R1.2, R1.3, R1.4, R1.5)
func TestCatDefaultBehavior(t *testing.T) {
	skipIfMissing(t)

	dir := t.TempDir()
	writeFile(t, dir, "file1.txt", []byte("hello\nworld\n"))
	writeFile(t, dir, "file2.txt", []byte("foo\nbar\n"))
	writeFile(t, dir, "noterminal.txt", []byte("no newline at end"))

	tests := []testutils.DiffTest{
		{
			Name:     "stdin_no_args",
			Stdin:    []byte("from stdin\n"),
			ExitCode: 0,
		},
		{
			Name:     "single_file",
			Args:     []string{"file1.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "multiple_files",
			Args:     []string{"file1.txt", "file2.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "stdin_dash",
			Args:     []string{"-"},
			Stdin:    []byte("from stdin\n"),
			ExitCode: 0,
		},
		{
			Name:     "dash_interleaved_with_files",
			Args:     []string{"file1.txt", "-", "file2.txt"},
			Stdin:    []byte("middle\n"),
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "double_dash_end_of_flags",
			Args:     []string{"--", "file1.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "no_trailing_newline",
			Args:     []string{"noterminal.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "empty_stdin",
			Stdin:    []byte(""),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestCatBinaryPassthrough tests that binary data passes through without
// corruption when no transformation flags are active.
// (prd006-cat R1.4)
func TestCatBinaryPassthrough(t *testing.T) {
	skipIfMissing(t)

	// Build all 256 byte values.
	allBytes := make([]byte, 256)
	for i := range allBytes {
		allBytes[i] = byte(i)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "all_256_bytes",
			Stdin:    allBytes,
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestCatLineNumbering tests -n and -b flags for line numbering per
// prd006-cat R2.
// (prd006-cat R2.1, R2.2, R2.3, R2.4)
func TestCatLineNumbering(t *testing.T) {
	skipIfMissing(t)

	tests := []testutils.DiffTest{
		{
			Name:     "n_numbers_all_lines",
			Args:     []string{"-n"},
			Stdin:    []byte("alpha\n\nbeta\n"),
			ExitCode: 0,
		},
		{
			Name:     "b_numbers_nonblank_lines",
			Args:     []string{"-b"},
			Stdin:    []byte("first\n\n\nsecond\n"),
			ExitCode: 0,
		},
		{
			Name:     "b_overrides_n",
			Args:     []string{"-b", "-n"},
			Stdin:    []byte("line1\n\nline2\n"),
			ExitCode: 0,
		},
		{
			Name:     "n_then_b_override",
			Args:     []string{"-n", "-b"},
			Stdin:    []byte("line1\n\nline2\n"),
			ExitCode: 0,
		},
		{
			Name:     "blank_is_newline_only",
			Args:     []string{"-b"},
			Stdin:    []byte("text\n \n\t\n\nmore\n"),
			ExitCode: 0,
		},
		{
			Name:     "n_numbering_across_files",
			Args:     []string{"-n", "-"},
			Stdin:    []byte("one\ntwo\n"),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestCatSqueezeBlanks tests -s flag for blank-line squeezing per
// prd006-cat R3.
// (prd006-cat R3.1, R3.2, R3.3)
func TestCatSqueezeBlanks(t *testing.T) {
	skipIfMissing(t)

	dir := t.TempDir()
	writeFile(t, dir, "endsblank.txt", []byte("aaa\n\n"))
	writeFile(t, dir, "startsblank.txt", []byte("\nbbb\n"))

	tests := []testutils.DiffTest{
		{
			Name:     "squeeze_consecutive_blanks",
			Args:     []string{"-s"},
			Stdin:    []byte("a\n\n\n\nb\n"),
			ExitCode: 0,
		},
		{
			Name:     "squeeze_many_blanks",
			Args:     []string{"-s"},
			Stdin:    []byte("\n\n\ntext\n\n\n\n\nmore\n\n\n"),
			ExitCode: 0,
		},
		{
			Name:     "squeeze_across_files",
			Args:     []string{"-s", "endsblank.txt", "startsblank.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "combined_ns",
			Args:     []string{"-n", "-s"},
			Stdin:    []byte("a\n\n\n\nb\n"),
			ExitCode: 0,
		},
		{
			Name:     "combined_bs",
			Args:     []string{"-b", "-s"},
			Stdin:    []byte("a\n\n\n\nb\n"),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestCatNonPrinting tests -v, -E, -T, -A, -e, -t, and -u flags per
// prd006-cat R4.
// (prd006-cat R4.1, R4.2, R4.3, R4.4, R4.5, R4.6, R4.7, R4.8, R4.9)
func TestCatNonPrinting(t *testing.T) {
	skipIfMissing(t)

	tests := []testutils.DiffTest{
		{
			Name:     "v_control_chars",
			Args:     []string{"-v"},
			Stdin:    []byte{0x01, 0x09, 0x1b, 0x7f, 0x80, 0xff},
			ExitCode: 0,
		},
		{
			Name:     "v_preserves_tab_newline",
			Args:     []string{"-v"},
			Stdin:    []byte("hello\tworld\n"),
			ExitCode: 0,
		},
		{
			Name:     "v_high_bit_chars",
			Args:     []string{"-v"},
			Stdin:    []byte{0x80, 0x81, 0x9f, 0xa0, 0xfe, 0xff, '\n'},
			ExitCode: 0,
		},
		{
			Name:     "E_shows_dollar_at_line_end",
			Args:     []string{"-E"},
			Stdin:    []byte("line one\nline two\n"),
			ExitCode: 0,
		},
		{
			Name:     "T_shows_tab_as_caret_I",
			Args:     []string{"-T"},
			Stdin:    []byte("col1\tcol2\tcol3\n"),
			ExitCode: 0,
		},
		{
			Name:     "A_equivalent_to_vET",
			Args:     []string{"-A"},
			Stdin:    []byte{0x01, '\t', 'h', 'e', 'l', 'l', 'o', '\n'},
			ExitCode: 0,
		},
		{
			Name:     "e_equivalent_to_vE",
			Args:     []string{"-e"},
			Stdin:    []byte{0x01, 'h', 'e', 'l', 'l', 'o', '\n'},
			ExitCode: 0,
		},
		{
			Name:     "t_equivalent_to_vT",
			Args:     []string{"-t"},
			Stdin:    []byte{0x01, '\t', 'h', 'e', 'l', 'l', 'o', '\n'},
			ExitCode: 0,
		},
		{
			Name:     "u_accepted_no_effect",
			Args:     []string{"-u"},
			Stdin:    []byte("test\n"),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestCatCombinedFlags tests combinations of transformation flags to verify
// the application order per prd006-cat R4.9.
// (prd006-cat R2.1, R3.3, R4.5, R4.9)
func TestCatCombinedFlags(t *testing.T) {
	skipIfMissing(t)

	tests := []testutils.DiffTest{
		{
			Name:     "vET_combined_separate_flags",
			Args:     []string{"-v", "-E", "-T"},
			Stdin:    []byte{0x01, '\t', 'h', 'i', '\n'},
			ExitCode: 0,
		},
		{
			Name:     "nsE_squeeze_number_ends",
			Args:     []string{"-n", "-s", "-E"},
			Stdin:    []byte("a\n\n\n\nb\n"),
			ExitCode: 0,
		},
		{
			Name:     "bsv_squeeze_number_nonprint",
			Args:     []string{"-b", "-s", "-v"},
			Stdin:    []byte{0x01, 'a', '\n', '\n', '\n', 0x02, 'b', '\n'},
			ExitCode: 0,
		},
		{
			Name:     "all_flags_nsAb",
			Args:     []string{"-n", "-s", "-A", "-b"},
			Stdin:    []byte{0x01, '\t', 'x', '\n', '\n', '\n', 'y', '\n'},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestCatErrors tests error handling and exit codes per prd006-cat R5.
// (prd006-cat R5.1, R5.2, R5.3)
func TestCatErrors(t *testing.T) {
	skipIfMissing(t)

	dir := t.TempDir()
	writeFile(t, dir, "real.txt", []byte("data\n"))

	tests := []testutils.DiffTest{
		{
			Name:      "missing_file_continues",
			Args:      []string{"nonexistent.txt", "real.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{progNameNormalizer},
		},
		{
			Name:      "missing_file_only",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{progNameNormalizer},
		},
		{
			Name:      "invalid_flag",
			Args:      []string{"-z"},
			Stdin:     []byte(""),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errPresenceNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
