// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var stderrNorm = regexp.MustCompile(`^g?comm:`)

func normStderr(b []byte) []byte {
	var out []byte
	for line := range bytes.SplitSeq(b, []byte("\n")) {
		if len(out) > 0 {
			out = append(out, '\n')
		}
		out = append(out, stderrNorm.ReplaceAll(line, []byte("PROG:"))...)
	}
	return out
}

func normStderrLower(b []byte) []byte {
	return bytes.ToLower(normStderr(b))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	writeFile(t, dir, "file1.txt", "a\nb\nc\n")
	writeFile(t, dir, "file2.txt", "b\nc\nd\n")
	writeFile(t, dir, "empty.txt", "")
	writeFile(t, dir, "single.txt", "x\n")
	writeFile(t, dir, "abc.txt", "a\nb\nc\nd\ne\n")
	writeFile(t, dir, "bdf.txt", "b\nd\nf\n")
	writeFile(t, dir, "all_same.txt", "a\nb\nc\n")
	writeFile(t, dir, "disjoint1.txt", "a\nc\ne\n")
	writeFile(t, dir, "disjoint2.txt", "b\nd\nf\n")
	writeFile(t, dir, "one_line_a.txt", "a\n")
	writeFile(t, dir, "one_line_b.txt", "b\n")
	writeFile(t, dir, "one_line_a_same.txt", "a\n")
	writeFile(t, dir, "caps.txt", "A\nB\nC\n")
	writeFile(t, dir, "lower.txt", "a\nb\nc\n")
	writeFile(t, dir, "multi.txt", "a\na\nb\nc\n")
	writeFile(t, dir, "multi2.txt", "a\nb\nb\nc\n")
	writeFile(t, dir, "noeol.txt", "a\nb\nc")
	writeFile(t, dir, "noeol2.txt", "b\nc\nd")
	writeFile(t, dir, "unsorted1.txt", "b\na\nc\n")
	writeFile(t, dir, "unsorted2.txt", "c\na\nb\n")
	writeFile(t, dir, "unsorted_both1.txt", "b\na\n")
	writeFile(t, dir, "unsorted_both2.txt", "c\na\n")

	env := []string{"LC_ALL=C"}

	tests := []testutils.DiffTest{
		{
			Name:    "r1_1_basic_three_columns",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_1_identical_files",
			Args:    []string{"all_same.txt", "all_same.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_2_interleaved",
			Args:    []string{"abc.txt", "bdf.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_2_disjoint_files",
			Args:    []string{"disjoint1.txt", "disjoint2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_2_case_sensitive",
			Args:    []string{"caps.txt", "lower.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_3_file1_empty",
			Args:    []string{"empty.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_3_file2_empty",
			Args:    []string{"file1.txt", "empty.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_3_both_empty",
			Args:    []string{"empty.txt", "empty.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_3_file1_exhausted_first",
			Args:    []string{"one_line_a.txt", "abc.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_3_file2_exhausted_first",
			Args:    []string{"abc.txt", "one_line_a.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_3_single_line_files",
			Args:    []string{"one_line_a.txt", "one_line_b.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_3_single_line_same",
			Args:    []string{"one_line_a.txt", "one_line_a_same.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_4_no_trailing_newline",
			Args:    []string{"noeol.txt", "noeol2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_4_mixed_eol",
			Args:    []string{"noeol.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_1_duplicate_lines_file1",
			Args:    []string{"multi.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_1_duplicate_lines_file2",
			Args:    []string{"file1.txt", "multi2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_2_single_vs_many",
			Args:    []string{"single.txt", "abc.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_1_stdin_as_file1",
			Args:    []string{"-", "file2.txt"},
			Stdin:   []byte("a\nb\nc\n"),
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_3_file1_longer",
			Args:    []string{"abc.txt", "single.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_1_suppress_col1",
			Args:    []string{"-1", "file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_1_suppress_col1_interleaved",
			Args:    []string{"-1", "abc.txt", "bdf.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_2_suppress_col2",
			Args:    []string{"-2", "file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_2_suppress_col2_disjoint",
			Args:    []string{"-2", "disjoint1.txt", "disjoint2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_3_suppress_col3",
			Args:    []string{"-3", "file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_3_suppress_col3_identical",
			Args:    []string{"-3", "all_same.txt", "all_same.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_4_suppress_12",
			Args:    []string{"-12", "file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_4_suppress_13",
			Args:    []string{"-13", "file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_4_suppress_23",
			Args:    []string{"-23", "file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_4_suppress_123",
			Args:    []string{"-123", "file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_4_suppress_separate_flags",
			Args:    []string{"-1", "-2", "file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_4_suppress_12_empty_file1",
			Args:    []string{"-12", "empty.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_4_suppress_1_noeol",
			Args:    []string{"-1", "noeol.txt", "noeol2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r2_1_suppress_col1_duplicates",
			Args:    []string{"-1", "multi.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:      "r3_1_default_order_file1_unsorted",
			Args:      []string{"unsorted1.txt", "file2.txt"},
			WorkDir:   dir,
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "r3_1_default_order_file2_unsorted",
			Args:      []string{"file1.txt", "unsorted2.txt"},
			WorkDir:   dir,
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "r3_1_default_order_both_unsorted",
			Args:      []string{"unsorted_both1.txt", "unsorted_both2.txt"},
			WorkDir:   dir,
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "r3_2_check_order_file1_unsorted",
			Args:      []string{"--check-order", "unsorted1.txt", "file2.txt"},
			WorkDir:   dir,
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "r3_2_check_order_file2_unsorted",
			Args:      []string{"--check-order", "file1.txt", "unsorted2.txt"},
			WorkDir:   dir,
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "r3_2_check_order_sorted_files",
			Args:      []string{"--check-order", "file1.txt", "file2.txt"},
			WorkDir:   dir,
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:    "r3_3_nocheck_order_unsorted",
			Args:    []string{"--nocheck-order", "unsorted1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r3_3_nocheck_order_both_unsorted",
			Args:    []string{"--nocheck-order", "unsorted_both1.txt", "unsorted_both2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r3_4_output_delimiter_pipe",
			Args:    []string{"--output-delimiter=|", "file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r3_4_output_delimiter_string",
			Args:    []string{"--output-delimiter=:::", "file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r3_4_output_delimiter_with_suppress",
			Args:    []string{"--output-delimiter=|", "-1", "file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r3_4_output_delimiter_space_syntax",
			Args:    []string{"--output-delimiter", "|", "file1.txt", "file2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:      "r4_2_nonexistent_file1",
			Args:      []string{"nonexistent.txt", "file2.txt"},
			WorkDir:   dir,
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderrLower},
		},
		{
			Name:      "r4_2_nonexistent_file2",
			Args:      []string{"file1.txt", "nonexistent.txt"},
			WorkDir:   dir,
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderrLower},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	lines := strings.Repeat("line\n", 500000)
	writeFile(t, dir, "big1.txt", lines)
	writeFile(t, dir, "big2.txt", lines)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, filepath.Join(dir, "big1.txt"), filepath.Join(dir, "big2.txt"))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		t.Fatal(err)
	}
	stdout.Close()
	err = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("comm timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
