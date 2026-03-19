// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd018-head R3.1–R3.5 (multi-file headers, error handling)
// and R4.1–R4.2 (exit codes).
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
		// --- Edge cases (R3.2, R3.3, R3.4) ---
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

		// --- Error conditions (R3.5, R4.1, R4.2) ---
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
			if tc.wantOut != "" && stdout.String() != tc.wantOut {
				t.Fatalf("stdout: got %q, want %q", stdout.String(), tc.wantOut)
			}
			if tc.wantErr != "" && !strings.Contains(stderr.String(), tc.wantErr) {
				t.Fatalf("stderr: got %q, want substring %q",
					stderr.String(), tc.wantErr)
			}
		})
	}
}
