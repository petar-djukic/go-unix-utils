// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/paste implementing prd027-paste R4.1-R4.4.
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

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}

// TestDiff runs differential tests against the gpaste reference binary.
// R4.1: parallel merge, R4.2: serial mode, R4.3: custom delimiters,
// R4.4: stdin as input via dash argument.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skip("reference binary gpaste not in PATH")
	}

	tests := []testutils.DiffTest{
		// R4.1: default parallel merge — stdin passthrough.
		{
			Name:     "stdin_passthrough",
			Args:     []string{},
			Stdin:    []byte("alpha\nbeta\n"),
			ExitCode: 0,
		},
		// R4.1: exit 0 on success.
		{
			Name:     "exit_zero_on_success",
			Args:     []string{"-"},
			Stdin:    []byte("hello\n"),
			ExitCode: 0,
		},
		// R4.2: serial mode with single stdin.
		{
			Name:     "serial_stdin_single",
			Args:     []string{"-s", "-"},
			Stdin:    []byte("a\nb\nc\n"),
			ExitCode: 0,
		},
		// R4.3: custom single-char delimiter with stdin.
		{
			Name:     "custom_delim_colon",
			Args:     []string{"-d:", "-", "-"},
			Stdin:    []byte("a\n1\nb\n2\n"),
			ExitCode: 0,
		},
		// R4.3: escape sequence — tab explicit.
		{
			Name:     "delim_escape_tab",
			Args:     []string{"-d\\t", "-", "-"},
			Stdin:    []byte("a\n1\nb\n2\n"),
			ExitCode: 0,
		},
		// R4.3: escape sequence — newline delimiter.
		{
			Name:     "delim_escape_newline",
			Args:     []string{"-d\\n", "-", "-"},
			Stdin:    []byte("a\n1\nb\n2\n"),
			ExitCode: 0,
		},
		// R4.3: escape sequence — backslash delimiter.
		{
			Name:     "delim_escape_backslash",
			Args:     []string{"-d\\\\", "-", "-"},
			Stdin:    []byte("a\n1\nb\n2\n"),
			ExitCode: 0,
		},
		// R4.3: escape sequence — empty delimiter (\0).
		{
			Name:     "delim_escape_empty",
			Args:     []string{"-d\\0", "-", "-"},
			Stdin:    []byte("a\n1\nb\n2\n"),
			ExitCode: 0,
		},
		// R4.4: single dash reads stdin.
		{
			Name:     "single_dash_stdin",
			Args:     []string{"-"},
			Stdin:    []byte("line1\nline2\n"),
			ExitCode: 0,
		},
		// R4.4: multiple dash arguments read stdin sequentially.
		{
			Name:     "multiple_dash_stdin",
			Args:     []string{"-", "-"},
			Stdin:    []byte("a\n1\nb\n2\n"),
			ExitCode: 0,
		},
		// R4.4: three dash arguments.
		{
			Name:     "three_dash_stdin",
			Args:     []string{"-", "-", "-"},
			Stdin:    []byte("a\n1\nx\nb\n2\ny\n"),
			ExitCode: 0,
		},
		// R4.2: serial mode with multiple dash arguments.
		{
			Name:     "serial_multiple_dash",
			Args:     []string{"-s", "-", "-"},
			Stdin:    []byte("a\nb\nc\nd\ne\nf\n"),
			ExitCode: 0,
		},
		// R4.3: cycling delimiter list.
		{
			Name:     "delim_cycling_list",
			Args:     []string{"-d:,", "-", "-", "-"},
			Stdin:    []byte("a\n1\nx\nb\n2\ny\n"),
			ExitCode: 0,
		},
		// R4.3: cycling delimiter in serial mode.
		{
			Name:     "serial_delim_cycling",
			Args:     []string{"-s", "-d:,"},
			Stdin:    []byte("a\nb\nc\nd\n"),
			ExitCode: 0,
		},
		// Empty input.
		{
			Name:     "empty_input",
			Args:     []string{},
			Stdin:    []byte{},
			ExitCode: 0,
		},
		// Single line no trailing newline.
		{
			Name:     "single_line_no_newline",
			Args:     []string{"-"},
			Stdin:    []byte("abc"),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFiles tests parallel merge with actual files.
// R4.1: default parallel merge of two files with tab delimiter.
func TestDiffFiles(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skip("reference binary gpaste not in PATH")
	}

	dir := t.TempDir()
	file1 := filepath.Join(dir, "f1.txt")
	file2 := filepath.Join(dir, "f2.txt")
	file3 := filepath.Join(dir, "f3.txt")
	writeTestFile(t, file1, "a\nb\nc\n")
	writeTestFile(t, file2, "1\n2\n3\n")
	writeTestFile(t, file3, "x\ny\n")

	tests := []testutils.DiffTest{
		// R4.1: two files, equal length, tab delimiter.
		{
			Name:     "two_files_parallel",
			Args:     []string{file1, file2},
			ExitCode: 0,
		},
		// R4.1: unequal file lengths — shorter file contributes empty fields.
		{
			Name:     "unequal_length_files",
			Args:     []string{file1, file3},
			ExitCode: 0,
		},
		// R4.1: three files parallel.
		{
			Name:     "three_files_parallel",
			Args:     []string{file1, file2, file3},
			ExitCode: 0,
		},
		// R4.3: custom delimiter with files.
		{
			Name:     "files_custom_delim",
			Args:     []string{"-d:", file1, file2},
			ExitCode: 0,
		},
		// R4.3: cycling delimiter with three files.
		{
			Name:     "files_cycling_delim",
			Args:     []string{"-d:,", file1, file2, file3},
			ExitCode: 0,
		},
		// R4.2: serial mode with single file.
		{
			Name:     "serial_single_file",
			Args:     []string{"-s", file1},
			ExitCode: 0,
		},
		// R4.2: serial mode with two files.
		{
			Name:     "serial_two_files",
			Args:     []string{"-s", file1, file2},
			ExitCode: 0,
		},
		// R4.2: serial mode with custom delimiter.
		{
			Name:     "serial_custom_delim",
			Args:     []string{"-s", "-d:", file1},
			ExitCode: 0,
		},
		// R4.4: mix of file and stdin.
		{
			Name:     "file_and_stdin",
			Args:     []string{file1, "-"},
			Stdin:    []byte("1\n2\n3\n"),
			ExitCode: 0,
		},
		// R4.4: stdin between two files.
		{
			Name:     "stdin_between_files",
			Args:     []string{file1, "-", file2},
			Stdin:    []byte("X\nY\nZ\n"),
			ExitCode: 0,
		},
		// Long delimiter flag form.
		{
			Name:     "long_delimiters_flag",
			Args:     []string{"--delimiters=:", file1, file2},
			ExitCode: 0,
		},
		// Long serial flag form.
		{
			Name:     "long_serial_flag",
			Args:     []string{"--serial", file1},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFileNotFound tests exit code on missing file (R4.2).
func TestDiffFileNotFound(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skip("reference binary gpaste not in PATH")
	}

	dir := t.TempDir()
	nonexistent := filepath.Join(dir, "nonexistent.txt")

	tests := []testutils.DiffTest{
		// R4.2: exit 1 when file cannot be opened.
		{
			Name:      "file_not_found",
			Args:      []string{nonexistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestVersion verifies that --version prints output and exits 0.
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
	if !strings.Contains(out, "paste") {
		t.Errorf("--version output does not contain 'paste': %q", out)
	}
}

// TestHelp verifies that --help prints usage and exits 0.
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
}
