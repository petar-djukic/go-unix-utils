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
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "a\nb\nc\n")
	writeFile(t, dir, "nums.txt", "1\n2\n")
	writeFile(t, dir, "short.txt", "x\n")
	writeFile(t, dir, "long.txt", "1\n2\n3\n4\n")

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?paste`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("paste"))
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
			Name:    "r4_1_exit_zero_parallel",
			Args:    []string{"a.txt", "nums.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r4_1_exit_zero_serial",
			Args:    []string{"-s", "a.txt"},
			WorkDir: dir,
		},
		{
			Name:  "r4_1_exit_zero_stdin",
			Args:  []string{"-"},
			Stdin: []byte("hello\nworld\n"),
		},
		{
			Name:    "r4_1_exit_zero_no_args_passthrough",
			Stdin:   []byte("line1\nline2\n"),
		},
		{
			Name:      "r4_2_nonexistent_file",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "r4_2_nonexistent_among_valid",
			Args:      []string{"a.txt", "nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "r4_2_serial_nonexistent",
			Args:      []string{"-s", "nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:    "r4_4_normal_with_sigpipe_handler",
			Args:    []string{"a.txt"},
			WorkDir: dir,
		},
		{
			Name:    "parallel_unequal_lengths",
			Args:    []string{"short.txt", "long.txt"},
			WorkDir: dir,
		},
		{
			Name:    "parallel_custom_delim",
			Args:    []string{"-d:", "a.txt", "nums.txt"},
			WorkDir: dir,
		},
		{
			Name:    "serial_two_files",
			Args:    []string{"-s", "a.txt", "nums.txt"},
			WorkDir: dir,
		},
		{
			Name:  "serial_stdin",
			Args:  []string{"-s", "-"},
			Stdin: []byte("x\ny\nz\n"),
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
