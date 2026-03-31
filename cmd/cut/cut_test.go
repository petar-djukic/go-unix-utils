// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/cut implementing prd026-cut R1-R4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// clearOutput normalizes output by discarding all content.
// Used for error tests where stderr messages differ between Go and GNU binaries
// but exit codes must match.
func clearOutput(b []byte) []byte {
	return nil
}

// TestDiff runs differential tests against the gcut reference binary.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skip("reference binary gcut not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: byte selection — single position.
		{
			Name:     "bytes_single_position",
			Args:     []string{"-b2"},
			Stdin:    []byte("abcdef\n"),
			ExitCode: 0,
		},
		// R1.1: byte selection — range N-M.
		{
			Name:     "bytes_range",
			Args:     []string{"-b2-4"},
			Stdin:    []byte("abcdef\n"),
			ExitCode: 0,
		},
		// R1.1: byte selection — from start (-M).
		{
			Name:     "bytes_from_start",
			Args:     []string{"-b-3"},
			Stdin:    []byte("abcdef\n"),
			ExitCode: 0,
		},
		// R1.1: byte selection — to end (N-).
		{
			Name:     "bytes_to_end",
			Args:     []string{"-b4-"},
			Stdin:    []byte("abcdef\n"),
			ExitCode: 0,
		},
		// R1.1: byte selection — comma-separated list.
		{
			Name:     "bytes_comma_list",
			Args:     []string{"-b1,3,5"},
			Stdin:    []byte("abcdef\n"),
			ExitCode: 0,
		},
		// R1.2: character selection equivalent to bytes under LC_ALL=C.
		{
			Name:     "chars_range",
			Args:     []string{"-c2-4"},
			Stdin:    []byte("abcdef\n"),
			ExitCode: 0,
		},
		// R1.4: line shorter than selected range.
		{
			Name:     "bytes_short_line",
			Args:     []string{"-b10"},
			Stdin:    []byte("abc\n"),
			ExitCode: 0,
		},
		// R1.4: multiple lines with varying lengths.
		{
			Name:     "bytes_multiline",
			Args:     []string{"-b2-4"},
			Stdin:    []byte("abcdef\nxy\n"),
			ExitCode: 0,
		},
		// R2.1: field selection with tab delimiter (default).
		{
			Name:     "fields_tab_default",
			Args:     []string{"-f2"},
			Stdin:    []byte("a\tb\tc\n"),
			ExitCode: 0,
		},
		// R2.1, R2.2: field selection with custom delimiter.
		{
			Name:     "fields_custom_delim",
			Args:     []string{"-d:", "-f2"},
			Stdin:    []byte("a:b:c\n"),
			ExitCode: 0,
		},
		// R2.1: multiple fields.
		{
			Name:     "fields_multiple",
			Args:     []string{"-d:", "-f1,3"},
			Stdin:    []byte("a:b:c\n"),
			ExitCode: 0,
		},
		// R2.3: only-delimited suppresses lines without delimiter.
		{
			Name:     "only_delimited_no_delim",
			Args:     []string{"-d:", "-f2", "-s"},
			Stdin:    []byte("no-delimiter\n"),
			ExitCode: 0,
		},
		// R2.3: only-delimited passes lines with delimiter.
		{
			Name:     "only_delimited_with_delim",
			Args:     []string{"-d:", "-f2", "-s"},
			Stdin:    []byte("a:b:c\n"),
			ExitCode: 0,
		},
		// R2.3: without -s, lines without delimiter printed unchanged.
		{
			Name:     "no_delim_without_s",
			Args:     []string{"-d:", "-f2"},
			Stdin:    []byte("no-delimiter\n"),
			ExitCode: 0,
		},
		// R2.4: output delimiter with fields.
		{
			Name:     "output_delim_fields",
			Args:     []string{"-d:", "-f1,3", "--output-delimiter=|"},
			Stdin:    []byte("a:b:c\n"),
			ExitCode: 0,
		},
		// R2.4: output delimiter with bytes.
		{
			Name:     "output_delim_bytes",
			Args:     []string{"-b1,3,5", "--output-delimiter=|"},
			Stdin:    []byte("abcdef\n"),
			ExitCode: 0,
		},
		// R3.1: complement with bytes.
		{
			Name:     "complement_bytes",
			Args:     []string{"--complement", "-b2-3"},
			Stdin:    []byte("abcdef\n"),
			ExitCode: 0,
		},
		// R3.3: complement with fields.
		{
			Name:     "complement_fields",
			Args:     []string{"--complement", "-d:", "-f2"},
			Stdin:    []byte("a:b:c\n"),
			ExitCode: 0,
		},
		// R3.1: complement with characters.
		{
			Name:     "complement_chars",
			Args:     []string{"--complement", "-c1,3"},
			Stdin:    []byte("abcdef\n"),
			ExitCode: 0,
		},
		// R4.3: zero-terminated mode with bytes.
		{
			Name:     "zero_terminated_bytes",
			Args:     []string{"-z", "-b2-4"},
			Stdin:    []byte("abcdef\x00xyz\x00"),
			ExitCode: 0,
		},
		// R4.3: zero-terminated mode with fields.
		{
			Name:     "zero_terminated_fields",
			Args:     []string{"-z", "-d:", "-f2"},
			Stdin:    []byte("a:b:c\x00d:e:f\x00"),
			ExitCode: 0,
		},
		// R4.3: zero-terminated with newlines in data.
		{
			Name:     "zero_terminated_embedded_newline",
			Args:     []string{"-z", "-b1-3"},
			Stdin:    []byte("ab\ncde\x00fgh\x00"),
			ExitCode: 0,
		},
		// R4.1: '-' reads stdin.
		{
			Name:     "dash_reads_stdin",
			Args:     []string{"-d:", "-f2", "-"},
			Stdin:    []byte("a:b:c\n"),
			ExitCode: 0,
		},
		// R4.4: exit 0 on success.
		{
			Name:     "exit_zero_on_success",
			Args:     []string{"-b1"},
			Stdin:    []byte("abc\n"),
			ExitCode: 0,
		},
		// Empty input.
		{
			Name:     "empty_input",
			Args:     []string{"-b1"},
			Stdin:    []byte{},
			ExitCode: 0,
		},
		// Error: no mode flag.
		{
			Name:      "no_mode_flag",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// Error: conflicting -b and -f.
		{
			Name:      "conflicting_b_and_f",
			Args:      []string{"-b1", "-f1"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// Error: conflicting -c and -f.
		{
			Name:      "conflicting_c_and_f",
			Args:      []string{"-c1", "-f1"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// Field range N- (to end of fields).
		{
			Name:     "fields_range_to_end",
			Args:     []string{"-d:", "-f2-"},
			Stdin:    []byte("a:b:c:d\n"),
			ExitCode: 0,
		},
		// Field range -M (from start).
		{
			Name:     "fields_range_from_start",
			Args:     []string{"-d:", "-f-2"},
			Stdin:    []byte("a:b:c:d\n"),
			ExitCode: 0,
		},
		// Complement with output delimiter.
		{
			Name:     "complement_output_delim",
			Args:     []string{"--complement", "-d:", "-f2", "--output-delimiter=|"},
			Stdin:    []byte("a:b:c\n"),
			ExitCode: 0,
		},
		// Long form flags.
		{
			Name:     "long_form_bytes",
			Args:     []string{"--bytes=2-4"},
			Stdin:    []byte("abcdef\n"),
			ExitCode: 0,
		},
		{
			Name:     "long_form_characters",
			Args:     []string{"--characters=1,3"},
			Stdin:    []byte("abcdef\n"),
			ExitCode: 0,
		},
		{
			Name:     "long_form_fields",
			Args:     []string{"--fields=2", "--delimiter=:"},
			Stdin:    []byte("a:b:c\n"),
			ExitCode: 0,
		},
		// Only-delimited long form.
		{
			Name:     "long_form_only_delimited",
			Args:     []string{"-d:", "-f1", "--only-delimited"},
			Stdin:    []byte("no-colon\na:b\n"),
			ExitCode: 0,
		},
		// Zero-terminated long form.
		{
			Name:     "long_form_zero_terminated",
			Args:     []string{"--zero-terminated", "-b1-2"},
			Stdin:    []byte("abc\x00def\x00"),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffMultipleFiles tests multiple file processing (R4.2).
func TestDiffMultipleFiles(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skip("reference binary gcut not in PATH")
	}

	dir := t.TempDir()
	file1 := filepath.Join(dir, "f1.txt")
	file2 := filepath.Join(dir, "f2.txt")
	writeTestFile(t, file1, "a:b:c\nx:y:z\n")
	writeTestFile(t, file2, "1:2:3\n4:5:6\n")

	tests := []testutils.DiffTest{
		// R4.2: multiple files processed in order.
		{
			Name:     "multiple_files",
			Args:     []string{"-d:", "-f2", file1, file2},
			ExitCode: 0,
		},
		// R4.2: mix of file and stdin.
		{
			Name:     "file_and_stdin",
			Args:     []string{"-d:", "-f1", file1, "-"},
			Stdin:    []byte("p:q:r\n"),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFileNotFound tests exit code on missing file (R4.4).
func TestDiffFileNotFound(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skip("reference binary gcut not in PATH")
	}

	dir := t.TempDir()
	validFile := filepath.Join(dir, "valid.txt")
	writeTestFile(t, validFile, "a:b:c\n")
	nonexistent := filepath.Join(dir, "nonexistent.txt")

	tests := []testutils.DiffTest{
		// R4.4: exit 1 when file cannot be opened.
		{
			Name:      "file_not_found",
			Args:      []string{"-d:", "-f2", nonexistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// R4.2, R4.4: one valid, one missing — processes valid, exits 1.
		{
			Name:      "mixed_valid_and_missing",
			Args:      []string{"-d:", "-f2", validFile, nonexistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}

// TestVersion verifies that --version prints output and exits 0.
// Not a differential test because version strings differ between implementations.
func TestVersion(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("--version exited with error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "cut") {
		t.Errorf("--version output does not contain 'cut': %q", out)
	}
}

// TestHelp verifies that --help prints usage and exits 0.
// Not a differential test because help text differs between implementations.
func TestHelp(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("--help exited with error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Usage:") {
		t.Errorf("--help output does not contain 'Usage:': %q", out)
	}
	if !strings.Contains(out, "zero-terminated") {
		t.Errorf("--help output does not mention zero-terminated: %q", out)
	}
}
