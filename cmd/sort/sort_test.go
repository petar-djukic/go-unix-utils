// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var progRe = regexp.MustCompile(`(?m)^(\S*/)?g?sort:`)
var tryRe = regexp.MustCompile(`'(\S*/)?g?sort --help'`)

func normStderr(b []byte) []byte {
	b = progRe.ReplaceAll(b, []byte("PROG:"))
	b = tryRe.ReplaceAll(b, []byte("'PROG --help'"))
	return b
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	writeFile(t, dir, "fruits.txt", "banana\napple\ncherry\n")
	writeFile(t, dir, "nums.txt", "10\n2\n1\n")
	writeFile(t, dir, "sorted.txt", "a\nb\nc\n")
	writeFile(t, dir, "unsorted.txt", "c\na\nb\n")
	writeFile(t, dir, "dups.txt", "a\na\nb\nb\nc\n")
	writeFile(t, dir, "fields.txt", "a 3\nb 1\nc 2\n")
	writeFile(t, dir, "colon.txt", "a:3\nb:1\nc:2\n")
	writeFile(t, dir, "blanks.txt", " banana\napple\n cherry\n")
	writeFile(t, dir, "months.txt", "MAR\nJAN\nDEC\nFEB\n")
	writeFile(t, dir, "versions.txt", "1.10\n1.2\n1.1\n")
	writeFile(t, dir, "human.txt", "1G\n2K\n3M\n100\n")
	writeFile(t, dir, "multi1.txt", "cherry\napple\n")
	writeFile(t, dir, "multi2.txt", "banana\ndate\n")
	writeFile(t, dir, "stable.txt", "b 2\na 1\nb 1\na 2\n")

	env := []string{"LC_ALL=C"}

	tests := []testutils.DiffTest{
		{
			Name:  "default_sort_stdin",
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   env,
		},
		{
			Name:    "default_sort_file",
			Args:    []string{"fruits.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:  "empty_input",
			Stdin: []byte(""),
			Env:   env,
		},
		{
			Name:  "single_line",
			Stdin: []byte("hello\n"),
			Env:   env,
		},
		{
			Name:  "reverse",
			Args:  []string{"-r"},
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   env,
		},
		{
			Name:  "numeric",
			Args:  []string{"-n"},
			Stdin: []byte("10\n2\n1\n"),
			Env:   env,
		},
		{
			Name:  "numeric_negative",
			Args:  []string{"-n"},
			Stdin: []byte("-5\n3\n-1\n10\n"),
			Env:   env,
		},
		{
			Name:  "reverse_numeric",
			Args:  []string{"-rn"},
			Stdin: []byte("10\n2\n1\n"),
			Env:   env,
		},
		{
			Name:  "human_numeric",
			Args:  []string{"-h"},
			Stdin: []byte("1G\n2K\n3M\n100\n"),
			Env:   env,
		},
		{
			Name:  "month",
			Args:  []string{"-M"},
			Stdin: []byte("MAR\nJAN\nDEC\nFEB\n"),
			Env:   env,
		},
		{
			Name:  "version",
			Args:  []string{"-V"},
			Stdin: []byte("1.10\n1.2\n1.1\n"),
			Env:   env,
		},
		{
			Name:  "unique",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "unique_numeric",
			Args:  []string{"-un"},
			Stdin: []byte("1\n02\n2\n3\n"),
			Env:   env,
		},
		{
			Name:  "stable",
			Args:  []string{"-s", "-k1,1"},
			Stdin: []byte("b 2\na 1\nb 1\na 2\n"),
			Env:   env,
		},
		{
			Name:  "key_field2",
			Args:  []string{"-k2,2"},
			Stdin: []byte("a 3\nb 1\nc 2\n"),
			Env:   env,
		},
		{
			Name:  "key_field2_numeric",
			Args:  []string{"-k2,2n"},
			Stdin: []byte("a 3\nb 1\nc 2\n"),
			Env:   env,
		},
		{
			Name:  "key_multi",
			Args:  []string{"-k1,1", "-k2,2n"},
			Stdin: []byte("a 3\na 1\nb 2\nb 1\n"),
			Env:   env,
		},
		{
			Name:  "key_reverse_modifier",
			Args:  []string{"-k2,2r"},
			Stdin: []byte("a 1\nb 3\nc 2\n"),
			Env:   env,
		},
		{
			Name:  "delimiter_colon",
			Args:  []string{"-t:", "-k2,2"},
			Stdin: []byte("a:3\nb:1\nc:2\n"),
			Env:   env,
		},
		{
			Name:  "delimiter_colon_numeric",
			Args:  []string{"-t:", "-k2,2n"},
			Stdin: []byte("x:10\ny:2\nz:1\n"),
			Env:   env,
		},
		{
			Name:  "ignore_blanks",
			Args:  []string{"-b"},
			Stdin: []byte(" banana\napple\n cherry\n"),
			Env:   env,
		},
		{
			Name:  "check_sorted",
			Args:  []string{"-c"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:      "check_unsorted",
			Args:      []string{"-c"},
			Stdin:     []byte("b\na\nc\n"),
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:  "check_quiet_sorted",
			Args:  []string{"-C"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "check_quiet_unsorted",
			Args:  []string{"-C"},
			Stdin: []byte("b\na\nc\n"),
			Env:   env,
		},
		{
			Name:  "check_numeric_sorted",
			Args:  []string{"-cn"},
			Stdin: []byte("1\n2\n10\n"),
			Env:   env,
		},
		{
			Name:      "check_numeric_unsorted",
			Args:      []string{"-cn"},
			Stdin:     []byte("1\n10\n2\n"),
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:  "check_unique_pass",
			Args:  []string{"-cu"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:      "check_unique_fail",
			Args:      []string{"-cu"},
			Stdin:     []byte("a\nb\nb\nc\n"),
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:  "check_reverse_sorted",
			Args:  []string{"-cr"},
			Stdin: []byte("c\nb\na\n"),
			Env:   env,
		},
		{
			Name:      "check_reverse_unsorted",
			Args:      []string{"-cr"},
			Stdin:     []byte("a\nb\nc\n"),
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:  "check_diagnose_first_long",
			Args:  []string{"--check=diagnose-first"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "check_silent_unsorted",
			Args:  []string{"--check=silent"},
			Stdin: []byte("b\na\nc\n"),
			Env:   env,
		},
		{
			Name:    "multi_file",
			Args:    []string{"multi1.txt", "multi2.txt"},
			WorkDir: dir,
			Env:     env,
		},
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("c\na\nb\n"),
			Env:   env,
		},
		{
			Name:      "invalid_long_flag",
			Args:      []string{"--invalid-option"},
			Env:       env,
			Normalize: []testutils.NormalizeFunc{normStderr},
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
