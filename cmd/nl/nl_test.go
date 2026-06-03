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
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:  "R1.1_default_numbering",
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.2_empty_lines_not_numbered",
			Stdin: []byte("a\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.2_all_empty_lines",
			Stdin: []byte("\n\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.1_single_line",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.2_mixed_empty_and_content",
			Stdin: []byte("\na\n\n\nb\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.1_trailing_no_newline",
			Stdin: []byte("a\nb"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffR2(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:  "R2.1_body_style_a",
			Args:  []string{"-b", "a"},
			Stdin: []byte("a\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_body_style_t",
			Args:  []string{"-b", "t"},
			Stdin: []byte("a\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_body_style_n",
			Args:  []string{"-b", "n"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_body_style_p_regex",
			Args:  []string{"-b", "p^[ab]"},
			Stdin: []byte("apple\nbanana\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.2_header_style_a",
			Args:  []string{"-h", "a"},
			Stdin: []byte("\\:\\:\\:\nheader1\nheader2\n\\:\\:\nbody1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.2_header_style_default_n",
			Stdin: []byte("\\:\\:\\:\nheader1\nheader2\n\\:\\:\nbody1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.3_footer_style_a",
			Args:  []string{"-f", "a"},
			Stdin: []byte("\\:\\:\\:\nheader1\n\\:\\:\nbody1\n\\:\nfooter1\nfooter2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.3_footer_style_default_n",
			Stdin: []byte("\\:\\:\\:\nheader1\n\\:\\:\nbody1\n\\:\nfooter1\nfooter2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.4_style_n_no_number_no_separator",
			Args:  []string{"-b", "n"},
			Stdin: []byte("line1\nline2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_R2.2_R2.3_all_sections_styled",
			Args:  []string{"-h", "a", "-b", "a", "-f", "a"},
			Stdin: []byte("\\:\\:\\:\nh1\nh2\n\\:\\:\nb1\nb2\n\\:\nf1\nf2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R2.1_body_style_p_no_match",
			Args:  []string{"-b", "p^z"},
			Stdin: []byte("apple\nbanana\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffR3(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:  "R3.1_format_ln",
			Args:  []string{"-n", "ln"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_format_rn",
			Args:  []string{"-n", "rn"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_format_rz",
			Args:  []string{"-n", "rz"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_format_ln_with_empty",
			Args:  []string{"-b", "a", "-n", "ln"},
			Stdin: []byte("a\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_format_rz_with_empty",
			Args:  []string{"-b", "a", "-n", "rz"},
			Stdin: []byte("a\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.2_width_3",
			Args:  []string{"-w", "3"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.2_width_10",
			Args:  []string{"-w", "10"},
			Stdin: []byte("a\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.2_width_1",
			Args:  []string{"-w", "1"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.3_separator_colon_space",
			Args:  []string{"-s", ": "},
			Stdin: []byte("a\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.3_separator_pipe",
			Args:  []string{"-s", "|"},
			Stdin: []byte("a\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.3_separator_empty",
			Args:  []string{"-s", ""},
			Stdin: []byte("a\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.4_start_value",
			Args:  []string{"-v", "10"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.4_increment",
			Args:  []string{"-i", "5"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.4_start_and_increment",
			Args:  []string{"-v", "10", "-i", "5"},
			Stdin: []byte("a\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.1_R3.2_R3.3_combined_AC2",
			Args:  []string{"-b", "a", "-n", "ln", "-w", "3", "-s", ": "},
			Stdin: []byte("a\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.2_R3.1_width_with_rz",
			Args:  []string{"-n", "rz", "-w", "4"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R3.3_separator_with_unnumbered",
			Args:  []string{"-s", ": "},
			Stdin: []byte("a\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffFiles(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	writeFixture(t, dir, "f1.txt", "a\nb\n")
	writeFixture(t, dir, "f2.txt", "c\nd\n")

	tests := []testutils.DiffTest{
		{
			Name:    "R1.4_continuous_across_files",
			Args:    []string{"f1.txt", "f2.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "R1.3_single_named_file",
			Args:    []string{"f1.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.dat")
	if err := os.WriteFile(largePath, bytes.Repeat([]byte("x\n"), 500000), 0o644); err != nil {
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
		t.Fatal("nl timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

func TestDiffR4(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:  "R4.1_section_delimiters_replaced",
			Args:  []string{"-b", "a", "-h", "a", "-f", "a"},
			Stdin: []byte("\\:\\:\\:\nheader1\n\\:\\:\nbody1\n\\:\nfooter1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.1_body_delimiter_only",
			Args:  []string{"-b", "a"},
			Stdin: []byte("line1\n\\:\\:\nline2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.1_footer_delimiter_only",
			Args:  []string{"-b", "a", "-f", "a"},
			Stdin: []byte("line1\n\\:\nfooter1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.1_multiple_pages",
			Args:  []string{"-b", "a", "-h", "a", "-f", "a"},
			Stdin: []byte("\\:\\:\\:\nh1\n\\:\\:\nb1\nb2\n\\:\nf1\n\\:\\:\\:\nh2\n\\:\\:\nb3\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.2_header_resets_counter",
			Args:  []string{"-b", "a", "-h", "a"},
			Stdin: []byte("\\:\\:\\:\nh1\nh2\n\\:\\:\nb1\nb2\n\\:\\:\\:\nh3\n\\:\\:\nb3\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.2_reset_with_custom_start",
			Args:  []string{"-b", "a", "-h", "a", "-v", "10"},
			Stdin: []byte("\\:\\:\\:\nh1\n\\:\\:\nb1\nb2\n\\:\\:\\:\nh2\n\\:\\:\nb3\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.2_reset_with_start_and_increment",
			Args:  []string{"-b", "a", "-h", "a", "-v", "5", "-i", "2"},
			Stdin: []byte("\\:\\:\\:\nh1\n\\:\\:\nb1\n\\:\\:\\:\nh2\n\\:\\:\nb2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.3_no_reset_with_p",
			Args:  []string{"-p", "-b", "a", "-h", "a", "-f", "a"},
			Stdin: []byte("\\:\\:\\:\nh1\nh2\n\\:\\:\nb1\nb2\n\\:\nf1\n\\:\\:\\:\nh3\n\\:\\:\nb3\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.3_no_reset_across_pages",
			Args:  []string{"-p", "-b", "a", "-h", "a"},
			Stdin: []byte("\\:\\:\\:\nh1\n\\:\\:\nb1\nb2\n\\:\\:\\:\nh2\n\\:\\:\nb3\nb4\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.4_join_blank_l2",
			Args:  []string{"-b", "a", "-l", "2"},
			Stdin: []byte("a\n\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.4_join_blank_l3",
			Args:  []string{"-b", "a", "-l", "3"},
			Stdin: []byte("a\n\n\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.4_join_blank_l1_default",
			Args:  []string{"-b", "a"},
			Stdin: []byte("a\n\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.4_join_blank_fewer_than_threshold",
			Args:  []string{"-b", "a", "-l", "3"},
			Stdin: []byte("a\n\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.1_R4.2_R4.3_combined_sections_with_p",
			Args:  []string{"-p", "-b", "a", "-h", "a", "-f", "a", "-v", "10", "-i", "5"},
			Stdin: []byte("\\:\\:\\:\nh1\n\\:\\:\nb1\n\\:\nf1\n\\:\\:\\:\nh2\n\\:\\:\nb2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R4.4_join_blank_with_body_t",
			Args:  []string{"-l", "2"},
			Stdin: []byte("a\n\n\nb\n\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
