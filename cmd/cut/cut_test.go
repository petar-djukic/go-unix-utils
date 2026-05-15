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
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	writeFile(t, dir, "fields.txt", "a:b:c\n")

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?cut`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("cut"))
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
			Name:  "r4_1_exit_zero_field_stdin",
			Args:  []string{"-d:", "-f2"},
			Stdin: []byte("a:b:c\n"),
		},
		{
			Name:  "r4_1_exit_zero_byte_stdin",
			Args:  []string{"-b2-4"},
			Stdin: []byte("abcdef\n"),
		},
		{
			Name:    "r4_1_exit_zero_file_arg",
			Args:    []string{"-d:", "-f1", "fields.txt"},
			WorkDir: dir,
		},
		{
			Name:  "r4_1_exit_zero_stdin_dash",
			Args:  []string{"-d:", "-f1", "-"},
			Stdin: []byte("x:y:z\n"),
		},
		{
			Name:      "r4_2_nonexistent_file",
			Args:      []string{"-d:", "-f1", "nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "r4_2_nonexistent_then_valid",
			Args:      []string{"-d:", "-f1", "nonexistent.txt", "fields.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "r4_2_valid_then_nonexistent",
			Args:      []string{"-d:", "-f1", "fields.txt", "nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:  "r4_3_large_output_no_write_error",
			Args:  []string{"-d:", "-f1,2,3"},
			Stdin: []byte("a:b:c\nd:e:f\ng:h:i\nj:k:l\nm:n:o\n"),
		},
		{
			Name:  "r4_4_normal_with_sigpipe_handler",
			Args:  []string{"-b1"},
			Stdin: []byte("abc\ndef\n"),
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
