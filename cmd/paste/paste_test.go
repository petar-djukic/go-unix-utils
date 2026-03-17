// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/paste against the GNU reference binary (gpaste).
// Implements prd027-paste R1.1-R1.4, R2.1-R2.3, R3.1-R3.3, R4.1-R4.4 test
// coverage: multi-file parallel merge with default tab delimiter, custom delimiter
// lists with cycling, escape sequences (\n, \t, \\, \0), stdin via '-' operand
// with round-robin consumption, unequal-length file handling, single-file
// passthrough, serial mode (-s) with default and custom delimiters, --version and
// --help output, invalid option error handling, and file-not-found error handling.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgNameNormalizer replaces the reference binary name (gpaste or its
// full path) with the Go binary name (paste) in stderr so error message
// comparisons match.
func stderrProgNameNormalizer(data []byte) []byte {
	// Replace full-path occurrences first (e.g., "/opt/homebrew/bin/gpaste:" → "paste:").
	for {
		idx := bytes.Index(data, []byte("/"))
		if idx < 0 {
			break
		}
		end := bytes.Index(data[idx:], []byte("gpaste:"))
		if end < 0 {
			break
		}
		data = append(data[:idx], append([]byte("paste:"), data[idx+end+len("gpaste:"):]...)...)
	}
	// Replace bare gpaste: occurrences.
	data = bytes.ReplaceAll(data, []byte("gpaste:"), []byte("paste:"))
	// Normalize "Try '/path/to/gpaste --help'" to "Try 'paste --help'".
	if idx := bytes.Index(data, []byte("Try '")); idx >= 0 {
		if end := bytes.Index(data[idx:], []byte("--help'")); end >= 0 {
			prefix := data[:idx]
			suffix := data[idx+end:]
			data = append(append(prefix, []byte("Try 'paste ")...), suffix...)
		}
	}
	return data
}

// stderrCaseNormalizer lowercases stderr so platform-specific error message
// casing differences do not cause false divergence.
func stderrCaseNormalizer(data []byte) []byte {
	return bytes.ToLower(data)
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	// Create temp files for file-input tests.
	tmpDir := t.TempDir()

	fileA := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(fileA, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	fileNums := filepath.Join(tmpDir, "nums.txt")
	if err := os.WriteFile(fileNums, []byte("1\n2\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Unequal-length files for R1.3 testing.
	fileShort := filepath.Join(tmpDir, "short.txt")
	if err := os.WriteFile(fileShort, []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	fileLong := filepath.Join(tmpDir, "long.txt")
	if err := os.WriteFile(fileLong, []byte("1\n2\n3\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	fileThree := filepath.Join(tmpDir, "three.txt")
	if err := os.WriteFile(fileThree, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	singleLine := filepath.Join(tmpDir, "single.txt")
	if err := os.WriteFile(singleLine, []byte("only\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	tests := []testutils.DiffTest{
		// --- R1.1: basic multi-file paste with tab delimiter (AC4) ---
		{
			Name: "two_files_tab_delim",
			Args: []string{fileA, fileNums},
		},
		{
			Name: "three_files_tab_delim",
			Args: []string{fileA, fileNums, fileThree},
		},

		// --- R1.2: default tab delimiter (AC4) ---
		{
			Name: "two_files_default_tab",
			Args: []string{fileA, fileNums},
		},

		// --- R1.3: unequal-length files (AC6) ---
		{
			Name: "unequal_short_first",
			Args: []string{fileShort, fileLong},
		},
		{
			Name: "unequal_long_first",
			Args: []string{fileLong, fileShort},
		},
		{
			Name: "empty_and_nonempty",
			Args: []string{emptyFile, fileA},
		},
		{
			Name: "nonempty_and_empty",
			Args: []string{fileA, emptyFile},
		},
		{
			Name: "three_files_unequal",
			Args: []string{fileShort, fileLong, fileA},
		},

		// --- R1.4: single-file paste / passthrough (AC7) ---
		{
			Name: "single_file_passthrough",
			Args: []string{fileA},
		},
		{
			Name: "single_file_multiline",
			Args: []string{fileThree},
		},

		// --- R1.4: stdin via '-' (AC5) ---
		{
			Name:  "stdin_single_dash",
			Args:  []string{"-"},
			Stdin: []byte("hello\nworld\n"),
		},
		{
			// No files given, reads stdin by default.
			Name:  "stdin_no_args",
			Stdin: []byte("hello\nworld\n"),
		},
		{
			// '-' and a named file.
			Name:  "stdin_dash_and_file",
			Args:  []string{"-", fileNums},
			Stdin: []byte("a\nb\n"),
		},
		{
			// Named file and '-'.
			Name:  "file_and_stdin_dash",
			Args:  []string{fileA, "-"},
			Stdin: []byte("1\n2\n"),
		},

		// --- R1.4: multiple '-' operands consuming round-robin (AC5) ---
		{
			Name:  "double_dash_round_robin",
			Args:  []string{"-", "-"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			Name:  "triple_dash_round_robin",
			Args:  []string{"-", "-", "-"},
			Stdin: []byte("1\n2\n3\n4\n5\n6\n"),
		},
		{
			// Multiple dash with file in between.
			Name:  "dash_file_dash",
			Args:  []string{"-", fileA, "-"},
			Stdin: []byte("x\ny\nz\nw\n"),
		},

		// --- R1.3: unequal with stdin ---
		{
			Name:  "stdin_unequal_stdin_shorter",
			Args:  []string{"-", fileLong},
			Stdin: []byte("x\n"),
		},
		{
			Name:  "stdin_unequal_file_shorter",
			Args:  []string{fileShort, "-"},
			Stdin: []byte("a\nb\nc\n"),
		},

		// --- R2.1: custom single-character delimiter (AC1) ---
		{
			Name: "comma_delimiter_two_files",
			Args: []string{"-d", ",", fileA, fileNums},
		},
		{
			Name: "colon_delimiter_two_files",
			Args: []string{"-d", ":", fileA, fileNums},
		},
		{
			Name: "space_delimiter_two_files",
			Args: []string{"-d", " ", fileA, fileNums},
		},

		// --- R2.1/R2.3: multi-character delimiter cycling (AC2) ---
		{
			Name: "cycling_delimiters_three_files",
			Args: []string{"-d", ",;:", fileA, fileNums, fileThree},
		},
		{
			Name: "cycling_delimiters_more_fields_than_delims",
			Args: []string{"-d", ",;", fileA, fileNums, fileThree, fileLong},
		},
		{
			Name: "cycling_delimiters_two_char_three_files",
			Args: []string{"-d", ",-", fileA, fileNums, fileThree},
		},

		// --- R2.2: escape sequences (AC3-AC6) ---
		{
			Name: "newline_delimiter",
			Args: []string{"-d", `\n`, fileA, fileNums},
		},
		{
			Name: "tab_delimiter_explicit",
			Args: []string{"-d", `\t`, fileA, fileNums},
		},
		{
			Name: "backslash_delimiter",
			Args: []string{"-d", `\\`, fileA, fileNums},
		},
		{
			Name: "empty_delimiter_backslash_zero",
			Args: []string{"-d", `\0`, fileA, fileNums},
		},

		// --- R2.2: escape sequences in multi-char delimiter list ---
		{
			Name: "mixed_escape_cycling",
			Args: []string{"-d", `,\n`, fileA, fileNums, fileThree},
		},
		{
			Name: "backslash_zero_in_list",
			Args: []string{"-d", `,\0:`, fileA, fileNums, fileThree, fileLong},
		},

		// --- R2.1: custom delimiter with unequal files ---
		{
			Name: "comma_delimiter_unequal",
			Args: []string{"-d", ",", fileShort, fileLong},
		},

		// --- R3.1: serial mode with default tab delimiter (AC1, AC4) ---
		{
			Name: "serial_single_file_tab",
			Args: []string{"-s", fileThree},
		},
		{
			Name: "serial_two_files_tab",
			Args: []string{"-s", fileA, fileNums},
		},
		{
			Name: "serial_three_files_tab",
			Args: []string{"-s", fileA, fileNums, fileThree},
		},
		{
			Name: "serial_single_line_file",
			Args: []string{"-s", singleLine},
		},
		{
			Name: "serial_empty_file",
			Args: []string{"-s", emptyFile},
		},
		{
			Name: "serial_empty_and_nonempty",
			Args: []string{"-s", emptyFile, fileA},
		},

		// --- R3.2: serial mode with custom delimiter (AC2) ---
		{
			Name: "serial_comma_delimiter",
			Args: []string{"-s", "-d", ",", fileThree},
		},
		{
			Name: "serial_cycling_delimiter",
			Args: []string{"-s", "-d", ",;:", fileThree},
		},
		{
			Name: "serial_cycling_delimiter_multi_file",
			Args: []string{"-s", "-d", ",;", fileA, fileThree},
		},
		{
			Name: "serial_newline_escape_delimiter",
			Args: []string{"-s", "-d", `\n`, fileThree},
		},
		{
			Name: "serial_empty_escape_delimiter",
			Args: []string{"-s", "-d", `\0`, fileThree},
		},

		// --- R3.3: serial mode with stdin (AC3) ---
		{
			Name:  "serial_stdin_dash",
			Args:  []string{"-s", "-"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			Name:  "serial_stdin_comma",
			Args:  []string{"-s", "-d", ",", "-"},
			Stdin: []byte("x\ny\nz\n"),
		},
		{
			Name:  "serial_stdin_empty",
			Args:  []string{"-s", "-"},
			Stdin: []byte{},
		},

		// --- R2.1: delimiter with --delimiters= long form ---
		{
			Name: "long_form_delimiters_comma",
			Args: []string{"--delimiters=,", fileA, fileNums},
		},

		// --- R2.1: delimiter with -d attached ---
		{
			Name: "short_d_attached_comma",
			Args: []string{"-d,", fileA, fileNums},
		},

		// --- R2.1: combined flags -sd ---
		{
			Name: "combined_sd_comma",
			Args: []string{"-sd", ",", fileThree},
		},

		// --- R2.1: custom delimiter with stdin ---
		{
			Name:  "comma_delimiter_stdin",
			Args:  []string{"-d", ",", "-", "-"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},

		// --- Edge cases ---
		{
			Name:  "empty_stdin",
			Args:  []string{"-"},
			Stdin: []byte{},
		},
		{
			Name: "two_empty_files",
			Args: []string{emptyFile, emptyFile},
		},
		{
			Name: "single_line_file",
			Args: []string{singleLine, fileA},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHelpVersion tests --help and --version output.
// These are not compared against the reference binary since output text differs.
// Instead we verify the exit code is 0 and output goes to stdout.
// R4.1, R4.2.
func TestDiffHelpVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// R4.1: --version exits 0 and prints version info to stdout.
	t.Run("version_exit_0", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(goBin, "--version")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("--version failed: %v", err)
		}
		if len(out) == 0 {
			t.Fatal("--version produced no output")
		}
		if !bytes.Contains(out, []byte("paste")) {
			t.Fatalf("--version output missing 'paste': %s", out)
		}
	})

	// R4.2: --help exits 0 and prints usage to stdout.
	t.Run("help_exit_0", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(goBin, "--help")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("--help failed: %v", err)
		}
		if len(out) == 0 {
			t.Fatal("--help produced no output")
		}
		if !bytes.Contains(out, []byte("Usage:")) {
			t.Fatalf("--help output missing 'Usage:': %s", out)
		}
	})
}

// TestDiffErrors tests error handling via differential testing.
// R4.3: invalid options. R4.4: file-not-found errors.
func TestDiffErrors(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	tmpDir := t.TempDir()

	fileA := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(fileA, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	nonexistentFile := filepath.Join(tmpDir, "nonexistent.txt")

	tests := []testutils.DiffTest{
		// R4.3: invalid option -x.
		{
			Name:      "error_invalid_option",
			Args:      []string{"-x"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R4.3: invalid long option.
		{
			Name:      "error_invalid_long_option",
			Args:      []string{"--invalid"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R4.4: nonexistent file only.
		{
			Name:      "error_nonexistent_file",
			Args:      []string{nonexistentFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer, stderrCaseNormalizer},
		},
		// R4.4: nonexistent file among valid files — parallel mode halts.
		{
			Name:      "error_nonexistent_with_valid_parallel",
			Args:      []string{fileA, nonexistentFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer, stderrCaseNormalizer},
		},
		// R4.4: nonexistent file in serial mode.
		{
			Name:      "error_nonexistent_serial",
			Args:      []string{"-s", nonexistentFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer, stderrCaseNormalizer},
		},
		// R4.4: nonexistent among valid in serial mode — processing continues.
		{
			Name:      "error_nonexistent_serial_with_valid",
			Args:      []string{"-s", fileA, nonexistentFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer, stderrCaseNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
