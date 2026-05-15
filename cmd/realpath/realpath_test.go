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
	refBin, err := exec.LookPath("grealpath")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := setupTestDir(t)

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?realpath`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("realpath"))
	})
	lowerStderr := testutils.NormalizeFunc(func(b []byte) []byte {
		return bytes.ToLower(b)
	})
	discardStdout := testutils.NormalizeFunc(func([]byte) []byte { return nil })
	errNorm := []testutils.NormalizeFunc{normalizeBinaryName, lowerStderr}

	tests := []testutils.DiffTest{
		// R1.1: default resolution of existing paths
		{Name: "existing-file", Args: []string{filepath.Join(dir, "file.txt")}},
		{Name: "existing-dir", Args: []string{filepath.Join(dir, "subdir")}},
		{Name: "with-dotdot", Args: []string{filepath.Join(dir, "subdir", "..")}},
		{Name: "with-dot", Args: []string{filepath.Join(dir, ".", "file.txt")}},
		{Name: "relative-file", Args: []string{"file.txt"}, WorkDir: dir},
		{Name: "relative-subdir", Args: []string{"subdir"}, WorkDir: dir},
		// R1.1: symlink resolution
		{Name: "symlink-file", Args: []string{filepath.Join(dir, "link")}},
		{Name: "symlink-dir", Args: []string{filepath.Join(dir, "dirlink")}},
		// R1.1/R1.2: last component missing is OK in default mode
		{Name: "missing-last-default", Args: []string{filepath.Join(dir, "nonexistent")}},
		// R1.2: error when intermediate component missing
		{
			Name:      "missing-intermediate",
			Args:      []string{filepath.Join(dir, "nodir", "file")},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R1.3: -e with existing path
		{Name: "e-existing-file", Args: []string{"-e", filepath.Join(dir, "file.txt")}},
		{Name: "e-existing-dir", Args: []string{"-e", filepath.Join(dir, "subdir")}},
		// R1.3: -e errors when last component missing
		{
			Name:      "e-missing-last",
			Args:      []string{"-e", filepath.Join(dir, "nonexistent")},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R1.3: -e errors when intermediate missing
		{
			Name:      "e-missing-intermediate",
			Args:      []string{"-e", filepath.Join(dir, "nodir", "file")},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R1.3: long form
		{
			Name: "canonicalize-existing-long",
			Args: []string{"--canonicalize-existing", filepath.Join(dir, "file.txt")},
		},
		// R1.4: -m with missing components
		{Name: "m-all-missing", Args: []string{"-m", filepath.Join(dir, "x", "y", "z")}},
		{Name: "m-existing", Args: []string{"-m", filepath.Join(dir, "file.txt")}},
		{Name: "m-partial-missing", Args: []string{"-m", filepath.Join(dir, "subdir", "missing")}},
		// R1.4: long form
		{
			Name: "canonicalize-missing-long",
			Args: []string{"--canonicalize-missing", filepath.Join(dir, "a", "b")},
		},
		// Multiple operands: all succeed
		{
			Name: "multiple-ok",
			Args: []string{filepath.Join(dir, "file.txt"), filepath.Join(dir, "subdir")},
		},
		// Multiple operands: mixed results
		{
			Name: "multiple-mixed",
			Args: []string{
				filepath.Join(dir, "file.txt"),
				filepath.Join(dir, "nodir", "f"),
			},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// Error: no operand
		{Name: "no-operand", ExitCode: 1, Normalize: errNorm},
		// Error: unknown long flag
		{Name: "unknown-long-flag", Args: []string{"--bogus"}, ExitCode: 1, Normalize: errNorm},
		// --help and --version (discard stdout since text differs)
		{Name: "help", Args: []string{"--help"}, Normalize: []testutils.NormalizeFunc{discardStdout}},
		{Name: "version", Args: []string{"--version"}, Normalize: []testutils.NormalizeFunc{discardStdout}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "file.txt"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "subdir"), filepath.Join(dir, "dirlink")); err != nil {
		t.Fatal(err)
	}
	return dir
}
