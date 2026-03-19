// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd005-wc R1.1–R1.4 and R2.1–R2.4.
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
		// === R1.1–R1.4: default behavior ===

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

		// === R2.1: -l counts newlines ===

		{
			Name:  "r2.1_l_flag_stdin",
			Args:  []string{"-l"},
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name: "r2.1_l_flag_file",
			Args: []string{"-l", simple},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: no trailing newline — newline count is one less than logical lines.
		{
			Name:  "r2.1_l_no_trailing_newline",
			Args:  []string{"-l"},
			Stdin: []byte("foo\nbar"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.1_l_empty_stdin",
			Args:  []string{"-l"},
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -l with multiple files shows total.
		{
			Name: "r2.1_l_multiple_files",
			Args: []string{"-l", simple, oneline},
			Env:  []string{"LC_ALL=C"},
		},

		// === R2.2: -w counts words ===

		{
			Name:  "r2.2_w_flag_stdin",
			Args:  []string{"-w"},
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name: "r2.2_w_flag_file",
			Args: []string{"-w", simple},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.2_w_empty_stdin",
			Args:  []string{"-w"},
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: multiple spaces and tabs between words.
		{
			Name:  "r2.2_w_extra_whitespace",
			Args:  []string{"-w"},
			Stdin: []byte("  foo  \t bar  \n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name: "r2.2_w_multiple_files",
			Args: []string{"-w", simple, multiword},
			Env:  []string{"LC_ALL=C"},
		},

		// === R2.3: -c counts bytes ===

		{
			Name:  "r2.3_c_flag_stdin",
			Args:  []string{"-c"},
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name: "r2.3_c_flag_file",
			Args: []string{"-c", simple},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.3_c_empty_stdin",
			Args:  []string{"-c"},
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -c and -m together, -m takes precedence.
		{
			Name:  "r2.3_cm_precedence",
			Args:  []string{"-c", "-m"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.3_mc_precedence",
			Args:  []string{"-m", "-c"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// === R2.4: -m counts characters ===

		{
			Name:  "r2.4_m_flag_stdin",
			Args:  []string{"-m"},
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name: "r2.4_m_flag_file",
			Args: []string{"-m", simple},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name:  "r2.4_m_empty_stdin",
			Args:  []string{"-m"},
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},

		// === R2 combined flags ===

		// R2.6: combined -lw prints lines then words.
		{
			Name:  "r2_combined_lw",
			Args:  []string{"-lw"},
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.6: combined -lwc prints lines, words, bytes.
		{
			Name:  "r2_combined_lwc",
			Args:  []string{"-lwc"},
			Stdin: []byte("foo\nbar baz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Combined -wc prints words then bytes.
		{
			Name:  "r2_combined_wc",
			Args:  []string{"-wc"},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Combined flags with file.
		{
			Name: "r2_combined_lw_file",
			Args: []string{"-lw", simple},
			Env:  []string{"LC_ALL=C"},
		},
		// Combined flags with multiple files.
		{
			Name: "r2_combined_lc_multiple_files",
			Args: []string{"-lc", simple, oneline},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
