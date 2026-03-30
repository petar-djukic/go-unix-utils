// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/stat against gstat reference binary.
// Implements prd082-stat AC3, AC4, AC5, AC6.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normProgramName normalizes the binary name prefix in stderr messages
// so that "gstat:" and our "stat:" match during comparison.
func normProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gstat:"), []byte("stat:"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gstat")
	if err != nil {
		t.Skip("reference binary gstat not in PATH")
	}

	dir := t.TempDir()

	regFile := filepath.Join(dir, "regular.txt")
	if err := os.WriteFile(regFile, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	emptyFile := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(emptyFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	symLink := filepath.Join(dir, "link")
	if err := os.Symlink(regFile, symLink); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		{
			Name: "regular file default",
			Args: []string{regFile},
		},
		{
			Name: "empty file default",
			Args: []string{emptyFile},
		},
		{
			Name: "directory default",
			Args: []string{subDir},
		},
		{
			Name: "symlink default",
			Args: []string{symLink},
		},
		{
			Name: "dereference symlink",
			Args: []string{"-L", symLink},
		},
		{
			Name: "terse regular file",
			Args: []string{"-t", regFile},
		},
		{
			Name: "terse directory",
			Args: []string{"-t", subDir},
		},
		{
			Name: "filesystem default",
			Args: []string{"-f", dir},
		},
		{
			Name: "filesystem terse",
			Args: []string{"-f", "-t", dir},
		},
		{
			Name: "multiple files",
			Args: []string{regFile, subDir},
		},
		{
			Name: "combined flags -Lt",
			Args: []string{"-Lt", symLink},
		},
		{
			Name:      "nonexistent file",
			Args:      []string{filepath.Join(dir, "nonexistent")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgramName},
		},
		{
			Name:      "nonexistent with valid file",
			Args:      []string{filepath.Join(dir, "nonexistent"), regFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgramName},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
