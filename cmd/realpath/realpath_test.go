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
		// R3.3: multiple operands, mixed results
		{
			Name: "multiple-mixed",
			Args: []string{
				filepath.Join(dir, "file.txt"),
				filepath.Join(dir, "nodir", "f"),
			},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.1: no operand
		{Name: "no-operand", ExitCode: 1, Normalize: errNorm},
		// R3.2: unknown long flag
		{Name: "unknown-long-flag", Args: []string{"--bogus"}, ExitCode: 1, Normalize: errNorm},
		// R3.2: unknown short flag
		{Name: "unknown-short-flag", Args: []string{"-Z"}, ExitCode: 1, Normalize: errNorm},
		// R3.3: multiple operands all failing
		{
			Name: "multiple-all-fail",
			Args: []string{
				filepath.Join(dir, "nodir", "a"),
				filepath.Join(dir, "nodir", "b"),
			},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.3: three operands, middle one fails
		{
			Name: "multiple-middle-fail",
			Args: []string{
				filepath.Join(dir, "file.txt"),
				filepath.Join(dir, "nodir", "f"),
				filepath.Join(dir, "subdir"),
			},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R1.5: -s (strip/no-symlinks) does not resolve symlinks
		{Name: "s-symlink-file", Args: []string{"-s", filepath.Join(dir, "link")}},
		{Name: "s-symlink-dir", Args: []string{"-s", filepath.Join(dir, "dirlink")}},
		{Name: "s-existing-file", Args: []string{"-s", filepath.Join(dir, "file.txt")}},
		{Name: "s-with-dotdot", Args: []string{"-s", filepath.Join(dir, "subdir", "..")}},
		{Name: "s-missing-intermediate", Args: []string{"-s", filepath.Join(dir, "nodir", "file")}},
		{Name: "s-missing-last", Args: []string{"-s", filepath.Join(dir, "nonexistent")}},
		// R1.5: -s combined with -e
		{Name: "s-e-existing", Args: []string{"-s", "-e", filepath.Join(dir, "file.txt")}},
		{
			Name:      "s-e-missing",
			Args:      []string{"-s", "-e", filepath.Join(dir, "nonexistent")},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R1.5: -s combined with -m
		{Name: "s-m-missing", Args: []string{"-s", "-m", filepath.Join(dir, "x", "y")}},
		// R1.5: long forms
		{Name: "strip-long", Args: []string{"--strip", filepath.Join(dir, "link")}},
		{Name: "no-symlinks-long", Args: []string{"--no-symlinks", filepath.Join(dir, "link")}},
		// R2.1: --relative-to
		{
			Name: "relative-to-parent",
			Args: []string{"--relative-to=" + dir, filepath.Join(dir, "subdir")},
		},
		{
			Name: "relative-to-subdir",
			Args: []string{"--relative-to=" + filepath.Join(dir, "subdir"), filepath.Join(dir, "file.txt")},
		},
		{
			Name: "relative-to-space",
			Args: []string{"--relative-to", dir, filepath.Join(dir, "file.txt")},
		},
		// R2.2: --relative-base
		{
			Name: "relative-base-under",
			Args: []string{"--relative-base=" + dir, filepath.Join(dir, "subdir")},
		},
		{
			Name: "relative-base-outside",
			Args: []string{"--relative-base=" + filepath.Join(dir, "subdir"), filepath.Join(dir, "file.txt")},
		},
		{
			Name: "relative-base-exact",
			Args: []string{"--relative-base=" + dir, dir},
		},
		// R2.3: --relative-to + --relative-base combined
		{
			Name: "rel-to-and-base-under",
			Args: []string{
				"--relative-to=" + filepath.Join(dir, "subdir"),
				"--relative-base=" + dir,
				filepath.Join(dir, "file.txt"),
			},
		},
		{
			Name: "rel-to-and-base-outside",
			Args: []string{
				"--relative-to=" + filepath.Join(dir, "subdir"),
				"--relative-base=" + dir,
				"/",
			},
		},
		// R1.5+R2.1: -s combined with --relative-to
		{
			Name: "s-relative-to",
			Args: []string{"-s", "--relative-to=" + dir, filepath.Join(dir, "link")},
		},
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
