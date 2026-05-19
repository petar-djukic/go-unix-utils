// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

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
	errNorm := []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrCase, normalizeOpenPrefix}

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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
