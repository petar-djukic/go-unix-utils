// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gjoin")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	writeFile(t, dir, "f1.txt", "a 1\nb 2\nc 3\n")
	writeFile(t, dir, "f2.txt", "a X\nb Y\nc Z\n")
	writeFile(t, dir, "r21_f1.txt", "X a\nY b\nZ c\n")
	writeFile(t, dir, "r21_f2.txt", "a P\nb Q\nc R\n")
	writeFile(t, dir, "j2_1.txt", "M a\nN b\nO c\n")
	writeFile(t, dir, "j2_2.txt", "W a\nV b\nU c\n")
	writeFile(t, dir, "c1.txt", "a,1\nb,2\nc,3\n")
	writeFile(t, dir, "c2.txt", "a,X\nb,Y\nc,Z\n")
	writeFile(t, dir, "cf1.txt", "1,a\n2,b\n3,c\n")
	writeFile(t, dir, "dup2.txt", "a X\na Y\nb Z\n")
	writeFile(t, dir, "partial1.txt", "a 1\nc 3\n")
	writeFile(t, dir, "partial2.txt", "b 2\nc 4\n")

	tests := []testutils.DiffTest{
		{
			Name:    "default_join_first_field",
			Args:    []string{"f1.txt", "f2.txt"},
			WorkDir: dir,
		},
		{
			Name:    "default_unmatched_suppressed",
			Args:    []string{"partial1.txt", "partial2.txt"},
			WorkDir: dir,
		},
		{
			Name:    "default_duplicate_keys_file2",
			Args:    []string{"f1.txt", "dup2.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_1_field_selection",
			Args:    []string{"-1", "2", "-2", "1", "r21_f1.txt", "r21_f2.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_2_j_combined_field",
			Args:    []string{"-j", "2", "j2_1.txt", "j2_2.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_3_output_format",
			Args:    []string{"-o", "0,1.2,2.2", "f1.txt", "f2.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_3_output_reorder",
			Args:    []string{"-o", "2.2,0,1.2", "f1.txt", "f2.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_3_output_join_only",
			Args:    []string{"-o", "0", "f1.txt", "f2.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_4_comma_separator",
			Args:    []string{"-t", ",", "c1.txt", "c2.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_combined_sep_and_fields",
			Args:    []string{"-t", ",", "-1", "2", "-2", "1", "cf1.txt", "c2.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_combined_sep_and_output",
			Args:    []string{"-t", ",", "-o", "0,1.2,2.2", "c1.txt", "c2.txt"},
			WorkDir: dir,
		},
		{
			Name:    "stdin_as_file1",
			Args:    []string{"-", "f2.txt"},
			Stdin:   []byte("a 1\nb 2\nc 3\n"),
			WorkDir: dir,
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
