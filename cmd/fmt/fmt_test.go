// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/fmt.
// Covers prd070-fmt R1.1, R2.1, R3.1, R4.1.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeNonEmpty replaces any non-empty output with a fixed marker.
// Used for stderr where message format differs between Go and GNU.
func normalizeNonEmpty(b []byte) []byte {
	if len(b) > 0 {
		return []byte("ERROR\n")
	}
	return b
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfmt")
	if err != nil {
		t.Skip("reference binary gfmt not in PATH")
	}

	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "para.txt",
		"This is a short paragraph that fits in 75 chars easily.\n")
	writeTestFile(t, tmpDir, "multi.txt",
		"first file content here\n")
	writeTestFile(t, tmpDir, "multi2.txt",
		"second file content here\n")

	longLine := "word " +
		"word word word word word word word word word " +
		"word word word word word word word word word " +
		"word word word word word word word word word " +
		"word word word word word word word word word end"

	indented := "    first indented line that is long enough to require wrapping to test indentation preservation properly\n" +
		"        second line has deeper indent and also needs to be long enough to wrap around\n"

	tests := []testutils.DiffTest{
		{
			// R1.1: default 75-char formatting, short line unchanged
			Name:  "short_line_unchanged",
			Args:  []string{},
			Stdin: []byte("short line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: default 75-char formatting on long line
			Name:  "default_width_wrap",
			Args:  []string{},
			Stdin: []byte(longLine + "\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1: blank lines separate paragraphs
			Name:  "paragraph_boundary",
			Args:  []string{},
			Stdin: []byte("first paragraph words words words words words\n\nsecond paragraph words words words words words\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1: multiple blank lines
			Name:  "multiple_blank_lines",
			Args:  []string{},
			Stdin: []byte("para one\n\n\npara two\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.1: indentation preservation
			Name:  "indentation_preserved",
			Args:  []string{},
			Stdin: []byte(indented),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R3.1: single-line paragraph indent
			Name:  "single_line_indent",
			Args:  []string{},
			Stdin: []byte("   indented single line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1: read from file argument
			Name:  "file_input",
			Args:  []string{filepath.Join(tmpDir, "para.txt")},
			Stdin: nil,
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1: read from multiple files
			Name: "multi_file_input",
			Args: []string{
				filepath.Join(tmpDir, "multi.txt"),
				filepath.Join(tmpDir, "multi2.txt"),
			},
			Stdin: nil,
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1: "-" means stdin
			Name:  "dash_stdin",
			Args:  []string{"-"},
			Stdin: []byte("stdin input line\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1: missing file produces error exit code
			Name:      "missing_file_error",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			Stdin:     nil,
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("writeTestFile %s: %v", name, err)
	}
}
