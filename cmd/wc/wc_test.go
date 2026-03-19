// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd005-wc R1.1–R1.4: default wc behavior.
// All tests run with LC_ALL=C per R5.1.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writeTestFile: %v", err)
	}
	return p
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	dir := t.TempDir()
	simple := writeTestFile(t, dir, "simple.txt", "foo\nbar baz\n")
	oneline := writeTestFile(t, dir, "oneline.txt", "hello\n")
	empty := writeTestFile(t, dir, "empty.txt", "")
	multiword := writeTestFile(t, dir, "multiword.txt", "one two three\nfour five\n")

	tests := []testutils.DiffTest{
		// R1.1: default counts (lines, words, bytes) from stdin.
		{
			Name:  "r1.1_default_counts_stdin",
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: reads from stdin when no file arguments given.
		{
			Name:  "r1.2_stdin_no_args",
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: reads from named file.
		{
			Name: "r1.2_single_named_file",
			Args: []string{simple},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: output format — counts followed by filename.
		{
			Name: "r1.3_output_with_filename",
			Args: []string{oneline},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: stdin produces no filename in output.
		{
			Name:  "r1.3_stdin_no_filename",
			Stdin: []byte("a b c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: total line when multiple files given.
		{
			Name: "r1.4_multiple_files_total",
			Args: []string{simple, oneline},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: total with three files.
		{
			Name: "r1.4_three_files_total",
			Args: []string{simple, oneline, multiword},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1 + R1.2: empty stdin produces zero counts.
		{
			Name:  "r1.1_empty_stdin",
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: empty file produces zero counts.
		{
			Name: "r1.1_empty_file",
			Args: []string{empty},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: explicit "-" reads stdin with filename displayed.
		{
			Name:  "r1.2_explicit_dash_stdin",
			Args:  []string{"-"},
			Stdin: []byte("test\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: multiple files including empty.
		{
			Name: "r1.4_files_with_empty",
			Args: []string{simple, empty},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
