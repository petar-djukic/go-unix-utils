// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunexpand")
	if err != nil {
		t.Skip("reference binary gunexpand not found")
	}
	tests := []testutils.DiffTest{
		// R1.1–R1.4: default (leading-only) behavior
		{
			Name:  "leading_8_spaces_become_tab",
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "leading_4_spaces_stay",
			Stdin: []byte("    text\n"),
		},
		{
			Name:  "non_leading_spaces_unchanged_default",
			Stdin: []byte("a        b\n"),
		},
		{
			Name:  "mixed_leading_tab_and_spaces",
			Stdin: []byte("\t    text\n"),
		},
		{
			Name:  "leading_tab_preserved",
			Stdin: []byte("\ttext\n"),
		},
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		{
			Name:  "newline_only",
			Stdin: []byte("\n"),
		},
		{
			Name:  "no_whitespace",
			Stdin: []byte("hello\n"),
		},

		// R2.1: -a converts all runs of spaces to tabs at tab stops
		{
			Name:  "a_non_leading_8_spaces_to_tab",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\n"),
		},
		{
			Name:  "a_leading_spaces_still_converted",
			Args:  []string{"-a"},
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "a_multiple_runs_in_line",
			Args:  []string{"-a"},
			Stdin: []byte("a        b        c\n"),
		},
		{
			Name:  "a_spaces_at_various_positions",
			Args:  []string{"-a"},
			Stdin: []byte("ab      cd      ef\n"),
		},
		{
			Name:  "a_spaces_at_tab_boundary",
			Args:  []string{"-a"},
			Stdin: []byte("1234567 x\n"),
		},
		{
			Name:  "a_spaces_reaching_tab_stop_exactly",
			Args:  []string{"-a"},
			Stdin: []byte("12345   x\n"),
		},
		{
			Name:  "a_mixed_tabs_and_spaces_midline",
			Args:  []string{"-a"},
			Stdin: []byte("a\t        b\n"),
		},

		// R2.2: single space not reaching tab stop stays as space
		{
			Name:  "a_single_space_preserved",
			Args:  []string{"-a"},
			Stdin: []byte("a b\n"),
		},
		{
			Name:  "a_two_spaces_not_reaching_stop",
			Args:  []string{"-a"},
			Stdin: []byte("ab  cd\n"),
		},
		{
			Name:  "a_three_spaces_not_reaching_stop",
			Args:  []string{"-a"},
			Stdin: []byte("abc   def\n"),
		},
		{
			Name:  "a_single_space_between_words",
			Args:  []string{"-a"},
			Stdin: []byte("hello world\n"),
		},

		// R2.3: conversion continues past non-whitespace with -a
		{
			Name:  "a_processes_entire_line",
			Args:  []string{"-a"},
			Stdin: []byte("x        y        z\n"),
		},
		{
			Name:  "a_multiple_words_with_large_gaps",
			Args:  []string{"-a"},
			Stdin: []byte("aa              bb              cc\n"),
		},
		{
			Name:  "a_trailing_spaces",
			Args:  []string{"-a"},
			Stdin: []byte("text        \n"),
		},
		{
			Name:  "a_multiline",
			Args:  []string{"-a"},
			Stdin: []byte("a        b\nc        d\n"),
		},
		{
			Name:  "a_empty_input",
			Args:  []string{"-a"},
			Stdin: []byte(""),
		},
		{
			Name:  "a_only_spaces_16",
			Args:  []string{"-a"},
			Stdin: []byte("                \n"),
		},
		{
			Name:  "a_tab_stop_edge_7_spaces",
			Args:  []string{"-a"},
			Stdin: []byte("a       b\n"),
		},

		// R3.1: -t N sets uniform interval
		{
			Name:  "t4_leading_4_spaces",
			Args:  []string{"-t", "4"},
			Stdin: []byte("    text\n"),
		},
		{
			Name:  "t4_leading_8_spaces",
			Args:  []string{"-t", "4"},
			Stdin: []byte("        text\n"),
		},
		{
			Name:  "t4_leading_3_spaces_no_tab",
			Args:  []string{"-t", "4"},
			Stdin: []byte("   text\n"),
		},
		{
			Name:  "t4_non_leading_spaces",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a   b\n"),
		},
		{
			Name:  "t4_multiple_runs",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a   b   c\n"),
		},

		// R3.1: -t LIST sets absolute tab stop positions
		{
			Name:  "tlist_4_8_leading_spaces",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("    text\n"),
		},
		{
			Name:  "tlist_4_8_12_leading_12",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("            text\n"),
		},
		{
			Name:  "tlist_4_8_midline",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("a   b   c\n"),
		},

		// R3.2: past last explicit stop, spaces kept as-is
		{
			Name:  "tlist_4_8_past_last_stop",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("            text\n"),
		},
		{
			Name:  "tlist_4_past_last_nonleading",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a               b\n"),
		},
		{
			Name:  "tlist_4_8_spaces_beyond_stop",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("a       b       c\n"),
		},
		{
			Name:  "tlist_4_only_past_stop",
			Args:  []string{"-t", "4,8"},
			Stdin: []byte("1234567890          end\n"),
		},

		// R3.3: -t implies -a (no explicit -a needed)
		{
			Name:  "t4_implies_a_non_leading",
			Args:  []string{"-t4"},
			Stdin: []byte("a   b   c\n"),
		},
		{
			Name:  "tlist_implies_a",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("a   b       c\n"),
		},
		{
			Name:  "t4_implies_a_mixed",
			Args:  []string{"-t", "4"},
			Stdin: []byte("    a   b\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffExitCodes(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunexpand")
	if err != nil {
		t.Skip("reference binary gunexpand not found")
	}

	dir := t.TempDir()
	writeFixture(t, dir, "valid.txt", "        text\n")

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		{
			Name:    "R4.1_success_exit_0",
			Args:    []string{"valid.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:      "R4.2_nonexistent_file_exit_1",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
			Env:       []string{"LC_ALL=C"},
		},
		{
			Name:      "R4.2_nonexistent_with_valid_continues",
			Args:      []string{"nonexistent.txt", "valid.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
			Env:       []string{"LC_ALL=C"},
		},
		{
			Name:      "R4.2_valid_then_nonexistent",
			Args:      []string{"valid.txt", "nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
			Env:       []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.dat")
	if err := os.WriteFile(largePath, bytes.Repeat([]byte("        text\n"), 500000), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, largePath)
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
		t.Fatal("unexpand timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
