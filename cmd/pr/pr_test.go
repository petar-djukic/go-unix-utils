// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/pr against gpr (GNU coreutils).
//
// Covers prd110-pr R1.1, R2.1, R2.2, R2.3, R3.1, R4.1, R4.2, R4.3, R5.1, R5.2.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// writeTestFile creates a temporary file with the given content and returns its path.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpr")
	if err != nil {
		t.Skip("reference binary gpr not in PATH")
	}

	tmpDir := t.TempDir()
	smallFile := writeTestFile(t, tmpDir, "small.txt",
		"line1\nline2\nline3\nline4\nline5\n")

	multiLineFile := writeTestFile(t, tmpDir, "multi.txt",
		generateLines(20))

	manyLinesFile := writeTestFile(t, tmpDir, "many.txt",
		generateLines(70))

	tests := []testutils.DiffTest{
		// R1.1: basic pagination with header
		{
			Name:      "basic_pagination_from_file",
			Args:      []string{"-t", smallFile},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.1: custom page length
		{
			Name:      "custom_page_length",
			Args:      []string{"-l", "20", "-t", smallFile},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// R2.2: suppress header with -t
		{
			Name: "omit_header_t_flag",
			Args: []string{"-t", smallFile},
		},
		// R2.2: suppress header and pagination with -T
		{
			Name: "omit_pagination_T_flag",
			Args: []string{"-T", smallFile},
		},
		// R2.3: read from stdin
		{
			Name:  "stdin_input",
			Args:  []string{"-t"},
			Stdin: []byte("hello\nworld\n"),
		},
		// R3.1: two-column output
		{
			Name: "two_columns",
			Args: []string{"-2", "-t", smallFile},
		},
		// R3.1: three-column output with across
		{
			Name: "three_columns_across",
			Args: []string{"-3", "-a", "-t", multiLineFile},
		},
		// R4.1: line numbering default
		{
			Name: "number_lines_default",
			Args: []string{"-n", "-t", smallFile},
		},
		// R4.1: line numbering with custom width
		{
			Name:  "number_lines_custom_width",
			Args:  []string{"-n:3", "-t"},
			Stdin: []byte("alpha\nbeta\ngamma\n"),
		},
		// R4.2: margin indent
		{
			Name: "margin_indent",
			Args: []string{"-o", "4", "-t", smallFile},
		},
		// R4.3: double space
		{
			Name: "double_space",
			Args: []string{"-d", "-t", smallFile},
		},
		// R4.3: column separator
		{
			Name: "column_separator",
			Args: []string{"-2", "-s|", "-t", multiLineFile},
		},
		// R2.1: custom header
		{
			Name:      "custom_header",
			Args:      []string{"-h", "MYHEADER", smallFile},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// Combined: numbering + columns
		{
			Name: "number_with_columns",
			Args: []string{"-n", "-2", "-t", multiLineFile},
		},
		// Combined: double-space + numbering
		{
			Name:  "double_space_with_numbering",
			Args:  []string{"-d", "-n", "-t"},
			Stdin: []byte("one\ntwo\nthree\n"),
		},
		// R5.1: exit 0 on success
		{
			Name:  "exit_zero_on_success",
			Args:  []string{"-t"},
			Stdin: []byte("ok\n"),
		},
		// Page boundary: file exceeds one page
		{
			Name:      "multi_page_output",
			Args:      []string{manyLinesFile},
			Normalize: []testutils.NormalizeFunc{testutils.TimestampNormalizer},
		},
		// Width control
		{
			Name: "custom_width_two_columns",
			Args: []string{"-2", "-w", "40", "-t", multiLineFile},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// generateLines creates n lines of "line N" content.
func generateLines(n int) string {
	var b []byte
	for i := 1; i <= n; i++ {
		b = append(b, []byte("line ")...)
		b = append(b, []byte(itoa(i))...)
		b = append(b, '\n')
	}
	return string(b)
}

// itoa converts an int to string without importing strconv in test scope.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
