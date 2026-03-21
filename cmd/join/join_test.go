// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd069-join R1.1–R1.4, R2.1, R2.2, R2.4.
// R1.1: Join two sorted files on common field.
// R1.2: Default whitespace splitting, single-space output separator.
// R1.3: Unpaired lines suppressed by default.
// R1.4: "-" reads from stdin.
// R2.1: -1/-2 field selection.
// R2.2: -j combined field selection.
// R2.4: -t custom separator.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normProgName normalizes the program name prefix in stderr so that
// "gjoin:" and "join:" both become "join:" for comparison.
func normProgName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gjoin:"), []byte("join:"))
}

// normErrMsg normalizes system error message case differences between
// GNU strerror() and Go syscall.Errno.Error().
func normErrMsg(data []byte) []byte {
	return bytes.ToLower(data)
}

// writeFile creates a file with the given content in dir.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
	return path
}

// setupFiles creates two input files and returns their paths.
func setupFiles(t *testing.T, content1, content2 string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	f1 := writeFile(t, dir, "file1", content1)
	f2 := writeFile(t, dir, "file2", content2)
	return f1, f2
}

// joinCase defines a test case for the join differential test.
type joinCase struct {
	name     string
	args     []string
	content1 string
	content2 string
	stdin    []byte
	exitCode int
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gjoin")
	if err != nil {
		t.Skipf("reference binary gjoin not in PATH: %v", err)
	}

	cases := []joinCase{
		// R1.1: basic join on first field
		{
			name:     "basic_join_field1",
			content1: "a 1\nb 2\nc 3\n",
			content2: "a X\nb Y\nc Z\n",
		},
		// R1.1: partial match — only some keys common
		{
			name:     "partial_match",
			content1: "a 1\nb 2\nd 4\n",
			content2: "b X\nc Y\nd Z\n",
		},
		// R1.3: no common keys — no output
		{
			name:     "no_common_keys",
			content1: "a 1\nc 3\n",
			content2: "b 2\nd 4\n",
		},
		// R1.1: single matching line
		{
			name:     "single_match",
			content1: "key val1\n",
			content2: "key val2\n",
		},
		// R1.1: file1 empty
		{
			name:     "file1_empty",
			content1: "",
			content2: "a 1\nb 2\n",
		},
		// R1.1: file2 empty
		{
			name:     "file2_empty",
			content1: "a 1\nb 2\n",
			content2: "",
		},
		// R1.1: both files empty
		{
			name:     "both_empty",
			content1: "",
			content2: "",
		},
		// R1.1: many-to-many cross product
		{
			name:     "many_to_many",
			content1: "a 1\na 2\n",
			content2: "a X\na Y\n",
		},
		// R1.2: multiple fields per line
		{
			name:     "multiple_fields",
			content1: "a b c\nd e f\n",
			content2: "a x y\nd w z\n",
		},
		// R1.1: file1 exhausted first
		{
			name:     "file1_exhausted_first",
			content1: "a 1\n",
			content2: "a X\nb Y\nc Z\n",
		},
		// R1.1: file2 exhausted first
		{
			name:     "file2_exhausted_first",
			content1: "a 1\nb 2\nc 3\n",
			content2: "a X\n",
		},
		// R1.2: lines without trailing newline
		{
			name:     "no_trailing_newline",
			content1: "a 1\nb 2",
			content2: "a X\nb Y",
		},

		// R2.1: -1 field selection (join on field 2 of file1)
		{
			name:     "field1_selection",
			args:     []string{"-1", "2"},
			content1: "x a\ny b\n",
			content2: "a 1\nb 2\n",
		},
		// R2.1: -2 field selection (join on field 2 of file2)
		{
			name:     "field2_selection",
			args:     []string{"-2", "2"},
			content1: "a 1\nb 2\n",
			content2: "x a\ny b\n",
		},
		// R2.1: -1 and -2 together
		{
			name:     "field1_and_field2",
			args:     []string{"-1", "2", "-2", "2"},
			content1: "x a\ny b\n",
			content2: "p a\nq b\n",
		},
		// R2.2: -j sets both fields
		{
			name:     "j_combined_field",
			args:     []string{"-j", "2"},
			content1: "x a\ny b\n",
			content2: "p a\nq b\n",
		},

		// R2.4: -t comma separator
		{
			name:     "sep_comma",
			args:     []string{"-t", ","},
			content1: "a,1,x\nb,2,y\n",
			content2: "a,X,p\nb,Y,q\n",
		},
		// R2.4: -t with field selection
		{
			name:     "sep_with_field_select",
			args:     []string{"-t", ",", "-1", "2", "-2", "1"},
			content1: "x,a,1\ny,b,2\n",
			content2: "a,X\nb,Y\n",
		},
		// R2.4: -t tab separator
		{
			name:     "sep_tab",
			args:     []string{"-t", "\t"},
			content1: "a\t1\nb\t2\n",
			content2: "a\tX\nb\tY\n",
		},
		// R2.4: -t colon separator
		{
			name:     "sep_colon",
			args:     []string{"-t", ":"},
			content1: "a:1\nb:2\n",
			content2: "a:X\nb:Y\n",
		},

		// R1.4: stdin as file1
		{
			name:     "stdin_as_file1",
			args:     []string{"-"},
			content1: "", // unused, stdin from Stdin field
			content2: "a X\nb Y\n",
			stdin:    []byte("a 1\nb 2\n"),
		},

		// R1.1: case-sensitive join (LC_ALL=C)
		{
			name:     "case_sensitive",
			content1: "A 1\na 2\n",
			content2: "A X\na Y\n",
		},
	}

	stderrNorm := []testutils.NormalizeFunc{normProgName}
	tests := make([]testutils.DiffTest, 0, len(cases)+2)
	for _, tc := range cases {
		dt := buildDiffTest(t, tc, stderrNorm)
		tests = append(tests, dt)
	}

	// Error case: nonexistent file
	errDir := t.TempDir()
	validFile := writeFile(t, errDir, "valid", "a 1\nb 2\n")
	nonexistent := filepath.Join(errDir, "nonexistent")
	errNorm := []testutils.NormalizeFunc{normProgName, normErrMsg}
	tests = append(tests, testutils.DiffTest{
		Name:      "nonexistent_file",
		Args:      []string{nonexistent, validFile},
		ExitCode:  1,
		Normalize: errNorm,
	})

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildDiffTest converts a joinCase to a testutils.DiffTest,
// creating temp files for the input content.
func buildDiffTest(t *testing.T, tc joinCase, norms []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	fullArgs := make([]string, 0, len(tc.args)+2)
	fullArgs = append(fullArgs, tc.args...)
	if tc.stdin != nil {
		// stdin as file1: "-" is already in tc.args, add file2 path
		_, f2 := setupFiles(t, tc.content1, tc.content2)
		fullArgs = append(fullArgs, f2)
	} else {
		f1, f2 := setupFiles(t, tc.content1, tc.content2)
		fullArgs = append(fullArgs, f1, f2)
	}
	return testutils.DiffTest{
		Name:      tc.name,
		Args:      fullArgs,
		Stdin:     tc.stdin,
		ExitCode:  tc.exitCode,
		Normalize: norms,
	}
}
