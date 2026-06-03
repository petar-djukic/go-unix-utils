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

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	writeFile(t, dir, "abc.txt", "a\nb\nc\n")
	writeFile(t, dir, "dups.txt", "a\na\nb\na\n")
	writeFile(t, dir, "allsame.txt", "x\nx\nx\n")
	writeFile(t, dir, "nodups.txt", "a\nb\nc\n")
	writeFile(t, dir, "single.txt", "one\n")
	writeFile(t, dir, "empty.txt", "")
	writeFile(t, dir, "trailing.txt", "a\na\n")

	env := []string{"LC_ALL=C"}

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?uniq`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("uniq"))
	})
	normalizeErrCase := testutils.NormalizeFunc(func(b []byte) []byte {
		return bytes.ToLower(b)
	})
	normalizeOpenPrefix := testutils.NormalizeFunc(func(b []byte) []byte {
		return bytes.ReplaceAll(b, []byte(": open "), []byte(": "))
	})
	normalizeTryHelp := testutils.NormalizeFunc(func(b []byte) []byte {
		var out [][]byte
		for _, line := range bytes.Split(b, []byte("\n")) {
			if !bytes.HasPrefix(bytes.TrimSpace(bytes.ToLower(line)), []byte("try ")) {
				out = append(out, line)
			}
		}
		return bytes.Join(out, []byte("\n"))
	})
	errNorm := []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrCase, normalizeOpenPrefix, normalizeTryHelp}

	tests := []testutils.DiffTest{
		{
			Name:  "r1_1_adjacent_duplicates",
			Args:  []string{},
			Stdin: []byte("a\na\nb\na\n"),
			Env:   env,
		},
		{
			Name:  "r1_2_no_duplicates",
			Args:  []string{},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "r1_2_all_same",
			Args:  []string{},
			Stdin: []byte("x\nx\nx\n"),
			Env:   env,
		},
		{
			Name:  "r1_2_single_line",
			Args:  []string{},
			Stdin: []byte("one\n"),
			Env:   env,
		},
		{
			Name:  "r1_2_empty_input",
			Args:  []string{},
			Stdin: []byte(""),
			Env:   env,
		},
		{
			Name:    "r1_3_file_input",
			Args:    []string{"dups.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:    "r1_3_file_no_dups",
			Args:    []string{"nodups.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:  "r1_3_stdin_explicit_dash",
			Args:  []string{"-"},
			Stdin: []byte("a\na\nb\n"),
			Env:   env,
		},
		{
			Name:    "r1_3_output_file",
			Args:    []string{"dups.txt", "out.txt"},
			WorkDir: dir,
			Env:     env,
			ExpectedFiles: map[string][]byte{
				"out.txt": []byte("a\nb\na\n"),
			},
		},
		{
			Name:  "r1_4_case_sensitive",
			Args:  []string{},
			Stdin: []byte("A\na\nA\n"),
			Env:   env,
		},
		{
			Name:  "r1_1_long_runs",
			Args:  []string{},
			Stdin: []byte("a\na\na\na\nb\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "r1_2_trailing_duplicate",
			Args:  []string{},
			Stdin: []byte("a\na\n"),
			Env:   env,
		},
		{
			Name:      "r1_3_nonexistent_file",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: errNorm,
			Env:       env,
		},
		{
			Name:    "r1_3_empty_file",
			Args:    []string{"empty.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:  "r2_1_d_basic",
			Args:  []string{"-d"},
			Stdin: []byte("a\na\nb\na\n"),
			Env:   env,
		},
		{
			Name:  "r2_1_d_no_repeats",
			Args:  []string{"-d"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "r2_1_d_all_same",
			Args:  []string{"-d"},
			Stdin: []byte("x\nx\nx\n"),
			Env:   env,
		},
		{
			Name:  "r2_1_d_single",
			Args:  []string{"-d"},
			Stdin: []byte("one\n"),
			Env:   env,
		},
		{
			Name:  "r2_1_d_empty",
			Args:  []string{"-d"},
			Stdin: []byte(""),
			Env:   env,
		},
		{
			Name:  "r2_2_D_basic",
			Args:  []string{"-D"},
			Stdin: []byte("a\na\nb\na\n"),
			Env:   env,
		},
		{
			Name:  "r2_2_D_no_repeats",
			Args:  []string{"-D"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "r2_2_D_all_same",
			Args:  []string{"-D"},
			Stdin: []byte("x\nx\nx\n"),
			Env:   env,
		},
		{
			Name:  "r2_2_D_triple",
			Args:  []string{"-D"},
			Stdin: []byte("a\na\na\nb\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "r2_2_D_empty",
			Args:  []string{"-D"},
			Stdin: []byte(""),
			Env:   env,
		},
		{
			Name:  "r2_3_u_basic",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\na\n"),
			Env:   env,
		},
		{
			Name:  "r2_3_u_all_unique",
			Args:  []string{"-u"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "r2_3_u_all_same",
			Args:  []string{"-u"},
			Stdin: []byte("x\nx\nx\n"),
			Env:   env,
		},
		{
			Name:  "r2_3_u_single",
			Args:  []string{"-u"},
			Stdin: []byte("one\n"),
			Env:   env,
		},
		{
			Name:  "r2_3_u_empty",
			Args:  []string{"-u"},
			Stdin: []byte(""),
			Env:   env,
		},
		{
			Name:  "r2_4_c_basic",
			Args:  []string{"-c"},
			Stdin: []byte("a\na\nb\na\n"),
			Env:   env,
		},
		{
			Name:  "r2_4_c_all_same",
			Args:  []string{"-c"},
			Stdin: []byte("x\nx\nx\n"),
			Env:   env,
		},
		{
			Name:  "r2_4_c_no_repeats",
			Args:  []string{"-c"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "r2_4_c_single",
			Args:  []string{"-c"},
			Stdin: []byte("one\n"),
			Env:   env,
		},
		{
			Name:  "r2_4_c_empty",
			Args:  []string{"-c"},
			Stdin: []byte(""),
			Env:   env,
		},
		{
			Name:  "r2_4_cd_combined",
			Args:  []string{"-c", "-d"},
			Stdin: []byte("a\na\nb\na\n"),
			Env:   env,
		},
		{
			Name:  "r2_4_cu_combined",
			Args:  []string{"-cu"},
			Stdin: []byte("a\na\nb\na\n"),
			Env:   env,
		},
		{
			Name:  "r3_1_i_basic",
			Args:  []string{"-i"},
			Stdin: []byte("A\na\nb\n"),
			Env:   env,
		},
		{
			Name:  "r3_1_i_mixed_case_run",
			Args:  []string{"-i"},
			Stdin: []byte("Hello\nhello\nHELLO\nworld\n"),
			Env:   env,
		},
		{
			Name:  "r3_1_i_no_effect",
			Args:  []string{"-i"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "r3_1_i_empty",
			Args:  []string{"-i"},
			Stdin: []byte(""),
			Env:   env,
		},
		{
			Name:  "r3_1_id_combined",
			Args:  []string{"-id"},
			Stdin: []byte("A\na\nb\n"),
			Env:   env,
		},
		{
			Name:  "r3_1_ic_combined",
			Args:  []string{"-ic"},
			Stdin: []byte("A\na\nb\n"),
			Env:   env,
		},
		{
			Name:  "r3_1_iu_combined",
			Args:  []string{"-iu"},
			Stdin: []byte("A\na\nb\nB\n"),
			Env:   env,
		},
		{
			Name:  "r3_2_f_skip_one_field",
			Args:  []string{"-f", "1"},
			Stdin: []byte("a foo\nb foo\nc bar\n"),
			Env:   env,
		},
		{
			Name:  "r3_2_f_skip_two_fields",
			Args:  []string{"-f", "2"},
			Stdin: []byte("a b c\nx y c\na b d\n"),
			Env:   env,
		},
		{
			Name:  "r3_2_f_attached_value",
			Args:  []string{"-f1"},
			Stdin: []byte("a foo\nb foo\nc bar\n"),
			Env:   env,
		},
		{
			Name:  "r3_2_f_zero",
			Args:  []string{"-f", "0"},
			Stdin: []byte("a\na\nb\n"),
			Env:   env,
		},
		{
			Name:  "r3_2_f_exceeds_fields",
			Args:  []string{"-f", "5"},
			Stdin: []byte("a b\nc d\n"),
			Env:   env,
		},
		{
			Name:  "r3_2_f_leading_blanks",
			Args:  []string{"-f", "1"},
			Stdin: []byte("  a foo\n  b foo\n  c bar\n"),
			Env:   env,
		},
		{
			Name:  "r3_2_f_tab_separated",
			Args:  []string{"-f", "1"},
			Stdin: []byte("a\tfoo\nb\tfoo\nc\tbar\n"),
			Env:   env,
		},
		{
			Name:  "r3_3_s_skip_one_char",
			Args:  []string{"-s", "1"},
			Stdin: []byte("xfoo\nyfoo\nxbar\n"),
			Env:   env,
		},
		{
			Name:  "r3_3_s_attached_value",
			Args:  []string{"-s1"},
			Stdin: []byte("xfoo\nyfoo\nxbar\n"),
			Env:   env,
		},
		{
			Name:  "r3_3_s_exceeds_length",
			Args:  []string{"-s", "100"},
			Stdin: []byte("abc\nxyz\n"),
			Env:   env,
		},
		{
			Name:  "r3_3_s_zero",
			Args:  []string{"-s", "0"},
			Stdin: []byte("a\na\nb\n"),
			Env:   env,
		},
		{
			Name:  "r3_4_w_basic",
			Args:  []string{"-w", "3"},
			Stdin: []byte("abcX\nabcY\nxyzZ\n"),
			Env:   env,
		},
		{
			Name:  "r3_4_w_attached_value",
			Args:  []string{"-w3"},
			Stdin: []byte("abcX\nabcY\nxyzZ\n"),
			Env:   env,
		},
		{
			Name:  "r3_4_w_zero",
			Args:  []string{"-w", "0"},
			Stdin: []byte("abc\nxyz\n"),
			Env:   env,
		},
		{
			Name:  "r3_4_w_exceeds_length",
			Args:  []string{"-w", "100"},
			Stdin: []byte("abc\nabc\nxyz\n"),
			Env:   env,
		},
		{
			Name:  "r3_2_r3_3_f_s_combined",
			Args:  []string{"-f", "1", "-s", "1"},
			Stdin: []byte("a xfoo\nb yfoo\na xbar\n"),
			Env:   env,
		},
		{
			Name:  "r3_2_r3_4_f_w_combined",
			Args:  []string{"-f", "1", "-w", "2"},
			Stdin: []byte("a foX\nb foY\nc ba\n"),
			Env:   env,
		},
		{
			Name:  "r3_1_r3_2_i_f_combined",
			Args:  []string{"-i", "-f", "1"},
			Stdin: []byte("a Foo\nb foo\nc bar\n"),
			Env:   env,
		},
		{
			Name:  "r3_3_r3_4_s_w_combined",
			Args:  []string{"-s", "1", "-w", "2"},
			Stdin: []byte("xab1\nyab2\nzcd3\n"),
			Env:   env,
		},
		{
			Name:  "r3_1_r3_4_i_w_combined",
			Args:  []string{"-i", "-w", "1"},
			Stdin: []byte("Abc\nabc\nBbc\n"),
			Env:   env,
		},
		{
			Name:  "r3_2_f_with_d",
			Args:  []string{"-f", "1", "-d"},
			Stdin: []byte("a foo\nb foo\nc bar\nd bar\ne baz\n"),
			Env:   env,
		},
		{
			Name:  "r3_2_f_with_c",
			Args:  []string{"-f", "1", "-c"},
			Stdin: []byte("a foo\nb foo\nc bar\n"),
			Env:   env,
		},
		{
			Name:  "r4_1_exit_0_success",
			Args:  []string{},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:    "r4_1_exit_0_file",
			Args:    []string{"abc.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:      "r4_2_nonexistent_file",
			Args:      []string{"no_such_file.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: errNorm,
			Env:       env,
		},
		{
			Name:      "r4_2_extra_operand",
			Args:      []string{"abc.txt", "out2.txt", "extra.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: errNorm,
			Env:       env,
		},
		{
			Name:      "r4_2_invalid_option",
			Args:      []string{"-x"},
			Stdin:     []byte("a\n"),
			ExitCode:  1,
			Normalize: errNorm,
			Env:       env,
		},
		{
			Name:  "r4_3_large_output_no_write_error",
			Args:  []string{},
			Stdin: []byte(strings.Repeat("a\nb\n", 10000)),
			Env:   env,
		},
		{
			Name:  "r4_4_normal_with_sigpipe_handler",
			Args:  []string{},
			Stdin: []byte("a\na\nb\n"),
			Env:   env,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.dat")
	if err := os.WriteFile(largePath, []byte(strings.Repeat("a\nb\n", 500000)), 0o644); err != nil {
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
		t.Fatal("uniq timed out; SIGPIPE handler may not be installed")
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
