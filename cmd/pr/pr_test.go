// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd110-pr R1.1, R1.2, R1.3, R1.4, R3.1, R3.2.
// R1.1: default 66-line page layout with header/body/footer.
// R1.2: -l/--length page length, -h/--header custom header text.
// R1.3: -t/--omit-header suppress header/footer, -T/--omit-pagination.
// R1.4: read from FILE arguments or stdin when no file or "-" given.
// R3.1: exit 0 on success, exit 1 on any error.
// R3.2: SIGPIPE handling via pkg/sys.InstallSIGPIPEHandler.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeBinaryName replaces "gpr:" or "pr:" with "PROG:" in stderr
// so that program name differences do not cause false failures.
func normalizeBinaryName(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gpr:"), []byte("PROG:"))
	b = bytes.ReplaceAll(b, []byte("pr:"), []byte("PROG:"))
	return b
}

// normalizeOpenError normalizes Go-style "open <path>: <msg>" to
// "read error" style matching GNU pr error format.
var openErrorRe = regexp.MustCompile(`open ([^:]+): `)

func normalizeOpenError(b []byte) []byte {
	return openErrorRe.ReplaceAll(b, []byte("$1: "))
}

// normalizeErrorCase normalizes case differences in error messages.
func normalizeErrorCase(b []byte) []byte {
	return bytes.ToLower(b)
}

// normalizeTimestamp replaces the date/time in page headers so the
// comparison is not affected by when the test runs.
var timestampRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}`)

func normalizeTimestamp(b []byte) []byte {
	return timestampRe.ReplaceAll(b, []byte("DATE"))
}

// createTestFile writes content to a file in dir and returns the path.
func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// generateLines builds a string with n numbered lines.
func generateLines(n int) string {
	var buf bytes.Buffer
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&buf, "line %d\n", i)
	}
	return buf.String()
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpr")
	if err != nil {
		t.Skip("reference binary gpr not in PATH")
	}

	tmpDir := t.TempDir()
	smallFile := createTestFile(t, tmpDir, "small.txt", "line1\nline2\nline3\n")
	largeFile := createTestFile(t, tmpDir, "large.txt", generateLines(70))
	secondFile := createTestFile(t, tmpDir, "second.txt", "alpha\nbeta\n")

	errNorm := []testutils.NormalizeFunc{
		normalizeBinaryName,
		normalizeOpenError,
		normalizeErrorCase,
	}

	successNorm := []testutils.NormalizeFunc{
		normalizeTimestamp,
	}

	mixedNorm := append(successNorm, errNorm...)

	tests := []testutils.DiffTest{
		// --- R1.1: page layout ---

		// R1.1: Default page layout with header, body, footer.
		{
			Name:      "r1_1_default_page_layout",
			Args:      []string{smallFile},
			Normalize: successNorm,
		},

		// R1.1: Multi-page output with >56 body lines.
		{
			Name:      "r1_1_multipage",
			Args:      []string{largeFile},
			Normalize: successNorm,
		},

		// R1.1: Empty file produces one page with header and empty body.
		{
			Name:      "r1_1_empty_file",
			Args:      []string{"/dev/null"},
			Normalize: successNorm,
		},

		// --- R1.2: page length and custom header ---

		// R1.2: -l sets custom page length.
		{
			Name:      "r1_2_custom_length",
			Args:      []string{"-l", "20", smallFile},
			Normalize: successNorm,
		},

		// R1.2: --length=N long form.
		{
			Name:      "r1_2_length_long",
			Args:      []string{"--length=20", smallFile},
			Normalize: successNorm,
		},

		// R1.2: -h sets custom header text.
		{
			Name:      "r1_2_custom_header",
			Args:      []string{"-h", "MyHeader", smallFile},
			Normalize: successNorm,
		},

		// R1.2: --header=HEADER long form.
		{
			Name:      "r1_2_header_long",
			Args:      []string{"--header=MyHeader", smallFile},
			Normalize: successNorm,
		},

		// R1.2: -l and -h together.
		{
			Name:      "r1_2_length_and_header",
			Args:      []string{"-l", "20", "-h", "Custom", smallFile},
			Normalize: successNorm,
		},

		// --- R1.3: omit header/pagination ---

		// R1.3: -t suppresses header and footer.
		{
			Name:      "r1_3_omit_header_t",
			Args:      []string{"-t", smallFile},
			Normalize: successNorm,
		},

		// R1.3: --omit-header long form.
		{
			Name:      "r1_3_omit_header_long",
			Args:      []string{"--omit-header", smallFile},
			Normalize: successNorm,
		},

		// R1.3: -T suppresses header/footer and does not pad.
		{
			Name:      "r1_3_omit_pagination_T",
			Args:      []string{"-T", smallFile},
			Normalize: successNorm,
		},

		// R1.3: --omit-pagination long form.
		{
			Name:      "r1_3_omit_pagination_long",
			Args:      []string{"--omit-pagination", smallFile},
			Normalize: successNorm,
		},

		// R1.3: -t with large file (no page breaks).
		{
			Name:      "r1_3_omit_header_large",
			Args:      []string{"-t", largeFile},
			Normalize: successNorm,
		},

		// --- R1.4: file and stdin reading ---

		// R1.4: Stdin when no file arguments given.
		{
			Name:      "r1_4_stdin_no_args",
			Args:      []string{"-t"},
			Stdin:     []byte("hello\nworld\n"),
			Normalize: successNorm,
		},

		// R1.4: "-" as explicit stdin.
		{
			Name:      "r1_4_stdin_dash",
			Args:      []string{"-t", "-"},
			Stdin:     []byte("hello\nworld\n"),
			Normalize: successNorm,
		},

		// R1.4: Multiple file arguments.
		{
			Name:      "r1_4_multiple_files",
			Args:      []string{smallFile, secondFile},
			Normalize: successNorm,
		},

		// R1.4: Multiple files with -t.
		{
			Name:      "r1_4_multiple_files_t",
			Args:      []string{"-t", smallFile, secondFile},
			Normalize: successNorm,
		},

		// --- R3.1: exit codes ---

		// Success: reading a valid file exits 0.
		{
			Name:      "success_exit_0",
			Args:      []string{"-t", smallFile},
			Normalize: successNorm,
		},

		// Success: reading stdin exits 0.
		{
			Name:      "stdin_exit_0",
			Args:      []string{"-t"},
			Stdin:     []byte("hello\n"),
			Normalize: successNorm,
		},

		// Error: nonexistent file exits 1.
		{
			Name:      "nonexistent_file_exit_1",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: errNorm,
		},

		// Success: /dev/null as input exits 0.
		{
			Name:      "devnull_exit_0",
			Args:      []string{"-t", "/dev/null"},
			Normalize: successNorm,
		},

		// Success: empty stdin with -t exits 0.
		{
			Name:      "empty_stdin_t_exit_0",
			Args:      []string{"-t"},
			Stdin:     []byte(""),
			Normalize: successNorm,
		},

		// Error: multiple files with one nonexistent exits 1.
		{
			Name:      "mixed_valid_invalid_exit_1",
			Args:      []string{"-t", smallFile, filepath.Join(tmpDir, "missing.txt")},
			ExitCode:  1,
			Normalize: mixedNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
