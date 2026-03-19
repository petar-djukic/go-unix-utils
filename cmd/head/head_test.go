// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd018-head R3.5 (error handling edge cases), R4.1–R4.4
// (differential tests, flag combinations, version/help output, coverage suite).
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

// normalizeProgName replaces the reference binary name prefix (ghead:) with
// head: so that stderr error messages compare equal between the Go and GNU
// binaries. Applied to both stdout and stderr; stdout is unaffected because
// it never contains the program name prefix.
func normalizeProgName(data []byte) []byte {
	return []byte(strings.ReplaceAll(string(data), "ghead: ", "head: "))
}

// writeTestFile creates a file in dir with the given content and returns
// its absolute path.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", name, err)
	}
	return p
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghead")
	if err != nil {
		t.Skipf("reference binary ghead not in PATH: %v", err)
	}

	tmpDir := t.TempDir()

	// Create test fixtures.
	file20 := writeTestFile(t, tmpDir, "twenty.txt",
		"1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\n16\n17\n18\n19\n20\n")
	fileNoNL := writeTestFile(t, tmpDir, "nonewline.txt", "abc")
	fileEmpty := writeTestFile(t, tmpDir, "empty.txt", "")
	fileSmall := writeTestFile(t, tmpDir, "small.txt", "a\nb\n")
	fileBinary := writeTestFile(t, tmpDir, "binary.txt", "abcdefghijklmnopqrstuvwxyz")

	// Create a file with restricted permissions for permission error testing.
	fileNoRead := writeTestFile(t, tmpDir, "noread.txt", "secret\n")
	if err := os.Chmod(fileNoRead, 0o000); err != nil {
		t.Fatalf("chmod noread: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(fileNoRead, 0o644) // best-effort restore for cleanup
	})

	nonExistent := filepath.Join(tmpDir, "does_not_exist.txt")

	errNorm := []testutils.NormalizeFunc{normalizeProgName}

	tests := []testutils.DiffTest{
		// --- R4.1: core flag differential tests ---
		{
			Name: "default_10_lines",
			Args: []string{file20},
		},
		{
			Name: "explicit_n_5",
			Args: []string{"-n", "5", file20},
		},
		{
			Name: "explicit_n_attached",
			Args: []string{"-n5", file20},
		},
		{
			Name: "long_lines_flag",
			Args: []string{"--lines=5", file20},
		},
		{
			Name: "byte_count_5",
			Args: []string{"-c", "5", fileBinary},
		},
		{
			Name: "byte_count_attached",
			Args: []string{"-c5", fileBinary},
		},
		{
			Name: "long_bytes_flag",
			Args: []string{"--bytes=5", fileBinary},
		},
		{
			Name:  "stdin_default",
			Args:  nil,
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n"),
		},
		{
			Name:  "stdin_n_3",
			Args:  []string{"-n", "3"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
		},
		{
			Name:  "stdin_c_5",
			Args:  []string{"-c", "5"},
			Stdin: []byte("abcdefghijklmnop"),
		},
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("hello\nworld\n"),
		},
		{
			Name: "negative_n_5",
			Args: []string{"-n", "-5", file20},
		},
		{
			Name: "negative_c_10",
			Args: []string{"-c", "-10", fileBinary},
		},

		// --- R4.2: flag combinations and edge cases ---
		{
			Name: "empty_file",
			Args: []string{fileEmpty},
		},
		{
			Name: "file_no_trailing_newline",
			Args: []string{fileNoNL},
		},
		{
			Name: "zero_lines",
			Args: []string{"-n", "0", file20},
		},
		{
			Name: "zero_bytes",
			Args: []string{"-c", "0", file20},
		},
		{
			Name:     "negative_count_exceeds_file",
			Args:     []string{"-n", "-100", fileSmall},
			ExitCode: 0,
		},
		{
			Name:     "negative_byte_count_exceeds_file",
			Args:     []string{"-c", "-1000", fileSmall},
			ExitCode: 0,
		},
		{
			Name: "verbose_single_file",
			Args: []string{"-v", file20},
		},
		{
			Name: "quiet_multiple_files",
			Args: []string{"-q", file20, fileSmall},
		},
		{
			Name: "verbose_multiple_files",
			Args: []string{"-v", file20, fileSmall},
		},
		{
			Name: "single_line_file",
			Args: []string{fileSmall},
		},
		{
			Name: "multi_file_default",
			Args: []string{file20, fileSmall},
		},
		{
			Name: "multi_file_n_3",
			Args: []string{"-n", "3", file20, fileSmall},
		},
		{
			Name: "multi_file_c_5",
			Args: []string{"-c", "5", fileBinary, file20},
		},
		{
			Name: "last_wins_cn",
			Args: []string{"-c", "5", "-n", "3", file20},
		},
		{
			Name: "last_wins_nc",
			Args: []string{"-n", "3", "-c", "5", fileBinary},
		},
		{
			Name: "quiet_long_flag",
			Args: []string{"--quiet", file20, fileSmall},
		},
		{
			Name: "silent_long_flag",
			Args: []string{"--silent", file20, fileSmall},
		},
		{
			Name: "verbose_long_flag",
			Args: []string{"--verbose", fileSmall},
		},
		{
			Name: "double_dash_separator",
			Args: []string{"--", file20},
		},
		{
			Name: "n_1_line",
			Args: []string{"-n", "1", file20},
		},
		{
			Name: "byte_count_exceeds_file",
			Args: []string{"-c", "1000", fileBinary},
		},
		{
			Name: "lines_exceeds_file",
			Args: []string{"-n", "100", fileSmall},
		},
		{
			Name:  "multi_file_with_stdin",
			Args:  []string{file20, "-"},
			Stdin: []byte("stdin1\nstdin2\n"),
		},

		// --- R4.4: coverage suite (specific scenarios from requirement) ---
		{
			Name: "r4_4_negative_c_100",
			Args: []string{"-c", "-100", fileBinary},
		},
		{
			Name:  "r4_4_stdin_with_n",
			Args:  []string{"-n", "2", "-"},
			Stdin: []byte("one\ntwo\nthree\nfour\n"),
		},
		{
			Name: "r4_4_quiet_single_file",
			Args: []string{"-q", file20},
		},
		{
			Name: "r4_4_verbose_with_bytes",
			Args: []string{"-v", "-c", "10", fileBinary},
		},

		// --- R3.5: error handling edge cases ---
		{
			Name:      "nonexistent_file",
			Args:      []string{nonExistent},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "permission_denied",
			Args:      []string{fileNoRead},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "multi_file_one_missing",
			Args:      []string{file20, nonExistent, fileSmall},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "multi_file_permission_error",
			Args:      []string{fileSmall, fileNoRead, file20},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "all_files_missing",
			Args:      []string{nonExistent, filepath.Join(tmpDir, "also_missing.txt")},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "directory_as_input",
			Args:      []string{tmpDir},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "directory_between_files",
			Args:      []string{fileSmall, tmpDir, file20},
			ExitCode:  1,
			Normalize: errNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestRunUnit tests the run function directly for argument parsing edge cases.
func TestRunUnit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		stdin    string
		wantExit int
		wantOut  string
		wantErr  string
	}{
		{
			name:     "invalid_option",
			args:     []string{"-Z"},
			wantExit: 1,
			wantErr:  "invalid option",
		},
		{
			name:     "missing_n_argument",
			args:     []string{"-n"},
			wantExit: 1,
			wantErr:  "option requires an argument",
		},
		{
			name:     "missing_c_argument",
			args:     []string{"-c"},
			wantExit: 1,
			wantErr:  "option requires an argument",
		},
		{
			name:     "invalid_line_count",
			args:     []string{"-n", "abc"},
			wantExit: 1,
			wantErr:  "invalid number of lines",
		},
		{
			name:     "invalid_byte_count",
			args:     []string{"-c", "abc"},
			wantExit: 1,
			wantErr:  "invalid number of bytes",
		},
		{
			name:     "empty_stdin",
			args:     nil,
			stdin:    "",
			wantExit: 0,
			wantOut:  "",
		},
		{
			name:     "stdin_fewer_than_10_lines",
			args:     nil,
			stdin:    "a\nb\nc\n",
			wantExit: 0,
			wantOut:  "a\nb\nc\n",
		},
		// R4.3: version and help output
		{
			name:     "version_flag",
			args:     []string{"--version"},
			wantExit: 0,
			wantOut:  "head (go-unix-utils)\n",
		},
		{
			name:     "help_flag",
			args:     []string{"--help"},
			wantExit: 0,
			wantOut:  "Usage: head [OPTION]... [FILE]...\n",
		},
		{
			name:     "missing_lines_long_argument",
			args:     []string{"--lines"},
			wantExit: 1,
			wantErr:  "option requires an argument",
		},
		{
			name:     "missing_bytes_long_argument",
			args:     []string{"--bytes"},
			wantExit: 1,
			wantErr:  "option requires an argument",
		},
		{
			name:     "invalid_lines_long",
			args:     []string{"--lines=xyz"},
			wantExit: 1,
			wantErr:  "invalid number of lines",
		},
		{
			name:     "invalid_bytes_long",
			args:     []string{"--bytes=xyz"},
			wantExit: 1,
			wantErr:  "invalid number of bytes",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			stdin := strings.NewReader(tc.stdin)
			code := run(tc.args, stdin, &stdout, &stderr)
			if code != tc.wantExit {
				t.Fatalf("exit code: got %d, want %d (stderr: %s)",
					code, tc.wantExit, stderr.String())
			}
			if tc.wantOut != "" && !strings.HasPrefix(stdout.String(), tc.wantOut) {
				t.Fatalf("stdout: got %q, want prefix %q",
					stdout.String(), tc.wantOut)
			}
			if tc.wantErr != "" && !strings.Contains(stderr.String(), tc.wantErr) {
				t.Fatalf("stderr: got %q, want substring %q",
					stderr.String(), tc.wantErr)
			}
		})
	}
}
