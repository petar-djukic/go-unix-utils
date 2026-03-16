// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/nl against gnl (GNU coreutils).
// Implements prd022-nl R1.1-R1.4 test coverage.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skipf("reference binary gnl not in PATH: %v", err)
	}

	// Create test fixtures in a temp directory.
	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "three-lines.txt", "first\n\nsecond\n")
	writeTestFile(t, tmpDir, "abc.txt", "a\nb\nc\n")
	writeTestFile(t, tmpDir, "def.txt", "d\ne\nf\n")
	writeTestFile(t, tmpDir, "empty.txt", "")
	writeTestFile(t, tmpDir, "no-trailing-newline.txt", "hello\nworld")
	writeTestFile(t, tmpDir, "all-empty-lines.txt", "\n\n\n")
	writeTestFile(t, tmpDir, "mixed.txt", "alpha\n\nbeta\n\ngamma\n")

	tests := []testutils.DiffTest{
		// R1.1: default mode numbers non-empty body lines with width 6 + tab.
		{
			Name:  "R1.1_default_body_numbering",
			Stdin: []byte("first\n\nsecond\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: all non-empty lines numbered sequentially.
		{
			Name:  "R1.1_sequential_numbering",
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: empty lines pass through unnumbered.
		{
			Name:  "R1.2_empty_lines_unnumbered",
			Stdin: []byte("x\n\n\ny\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: input of only empty lines — none numbered.
		{
			Name:  "R1.2_all_empty_lines",
			Stdin: []byte("\n\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: read from stdin when no arguments.
		{
			Name:  "R1.3_stdin_no_args",
			Stdin: []byte("hello\nworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: "-" means stdin.
		{
			Name:  "R1.3_dash_means_stdin",
			Args:  []string{"-"},
			Stdin: []byte("foo\nbar\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: read from a named file.
		{
			Name:    "R1.2_named_file",
			Args:    []string{filepath.Join(tmpDir, "three-lines.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// R1.4: continuous numbering across multiple files.
		{
			Name: "R1.4_continuous_numbering_multifile",
			Args: []string{
				filepath.Join(tmpDir, "abc.txt"),
				filepath.Join(tmpDir, "def.txt"),
			},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// R1.4: nonexistent file exits >0 with error on stderr.
		{
			Name:      "R1.4_nonexistent_file",
			Args:      []string{filepath.Join(tmpDir, "does-not-exist.txt")},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R1.4: nonexistent mixed with existing files — exit 1, still
		// processes existing files with continuous numbering.
		{
			Name: "R1.4_nonexistent_mixed",
			Args: []string{
				filepath.Join(tmpDir, "abc.txt"),
				filepath.Join(tmpDir, "does-not-exist.txt"),
				filepath.Join(tmpDir, "def.txt"),
			},
			WorkDir:   tmpDir,
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// Edge: empty input from stdin.
		{
			Name:  "empty_stdin",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// Edge: empty file.
		{
			Name:    "empty_file",
			Args:    []string{filepath.Join(tmpDir, "empty.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// Edge: single line.
		{
			Name:  "single_line",
			Stdin: []byte("only\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Edge: file without trailing newline.
		{
			Name:    "no_trailing_newline",
			Args:    []string{filepath.Join(tmpDir, "no-trailing-newline.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// Edge: multiple empty lines interspersed with content.
		{
			Name:    "mixed_empty_and_content",
			Args:    []string{filepath.Join(tmpDir, "mixed.txt")},
			WorkDir: tmpDir,
			Env:     []string{"LC_ALL=C"},
		},
		// Edge: single empty line.
		{
			Name:  "single_empty_line",
			Stdin: []byte("\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", name, err)
	}
}

// normalizeProgramName normalizes error messages for differential comparison.
// GNU nl reports errors as "gnl: ..." while our binary uses "nl: ...". This
// normalizer replaces the program name and lowercases to eliminate both differences.
func normalizeProgramName(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gnl: "), []byte("nl: "))
	return bytes.ToLower(b)
}
