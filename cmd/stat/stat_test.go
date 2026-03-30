// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/stat against gstat reference binary.
// Implements prd082-stat AC1-AC5.
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
		// Default output tests (R1.1, R2.1, R2.2, R2.3)
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

		// Format string tests (R3.1)
		{
			Name: "format name and size",
			Args: []string{"-c", "%n %s", regFile},
		},
		{
			Name: "format permissions octal and human",
			Args: []string{"-c", "%a %A", regFile},
		},
		{
			Name: "format user and group names",
			Args: []string{"-c", "%U %G", regFile},
		},
		{
			Name: "format user and group ids",
			Args: []string{"-c", "%u %g", regFile},
		},
		{
			Name: "format inode and links",
			Args: []string{"-c", "%i %h", regFile},
		},
		{
			Name: "format file type",
			Args: []string{"-c", "%F", regFile},
		},
		{
			Name: "format directory type",
			Args: []string{"-c", "%F", subDir},
		},
		{
			Name: "format raw mode hex",
			Args: []string{"-c", "%f", regFile},
		},
		{
			Name: "format device decimal and hex",
			Args: []string{"-c", "%d %D", regFile},
		},
		{
			Name: "format blocks and block size",
			Args: []string{"-c", "%b %B %o", regFile},
		},
		{
			Name: "format symlink quoted name",
			Args: []string{"-c", "%N", symLink},
		},
		{
			Name: "format regular quoted name",
			Args: []string{"-c", "%N", regFile},
		},
		{
			Name: "format timestamps human",
			Args: []string{"-c", "%x|%y|%z", regFile},
		},
		{
			Name: "format timestamps epoch",
			Args: []string{"-c", "%X %Y %Z", regFile},
		},
		{
			Name: "format birth time",
			Args: []string{"-c", "%w %W", regFile},
		},
		{
			Name: "format device type major minor",
			Args: []string{"-c", "%t %T", regFile},
		},
		{
			Name: "format mount point",
			Args: []string{"-c", "%m", regFile},
		},
		{
			Name: "format percent literal",
			Args: []string{"-c", "%%", regFile},
		},
		{
			Name: "format long flag syntax",
			Args: []string{"--format=%n %s", regFile},
		},
		{
			Name: "format multiple files",
			Args: []string{"-c", "%n %s", regFile, subDir},
		},

		// Printf tests (R4.1, R4.2)
		{
			Name: "printf no trailing newline",
			Args: []string{"--printf=%n", regFile},
		},
		{
			Name: "printf with newline escape",
			Args: []string{"--printf=%n\\n", regFile},
		},
		{
			Name: "printf with tab escape",
			Args: []string{"--printf=%n\\t%s\\n", regFile},
		},
		{
			Name: "printf backslash literal",
			Args: []string{"--printf=%n\\\\%s\\n", regFile},
		},

		// Filesystem format tests (R5.1, R6.1)
		{
			Name: "fs format total blocks",
			Args: []string{"-f", "-c", "%b", dir},
		},
		{
			Name: "fs format total inodes",
			Args: []string{"-f", "-c", "%c", dir},
		},
		{
			Name: "fs format type name",
			Args: []string{"-f", "-c", "%T", dir},
		},
		{
			Name: "fs format name",
			Args: []string{"-f", "-c", "%n", dir},
		},
		{
			Name: "fs format block sizes",
			Args: []string{"-f", "-c", "%s %S", dir},
		},
		{
			Name: "fs format fsid",
			Args: []string{"-f", "-c", "%i", dir},
		},
		{
			Name: "fs format type hex",
			Args: []string{"-f", "-c", "%t", dir},
		},
		// Multi-file edge cases (R6.1, R7.1, R7.2)
		{
			Name: "three regular files",
			Args: []string{regFile, emptyFile, regFile},
		},
		{
			Name:      "all nonexistent",
			Args:      []string{filepath.Join(dir, "no1"), filepath.Join(dir, "no2")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgramName},
		},
		{
			Name:      "valid between nonexistent",
			Args:      []string{filepath.Join(dir, "no1"), regFile, filepath.Join(dir, "no2")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgramName},
		},
		{
			Name: "filesystem format free blocks and inodes",
			Args: []string{"-f", "-c", "%a %d %f", dir},
		},
		{
			Name: "filesystem format max namelen",
			Args: []string{"-f", "-c", "%l", dir},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestVersion verifies --version prints output and exits 0.
func TestVersion(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("--version produced no output")
	}
}

// TestHelp verifies --help prints output and exits 0.
func TestHelp(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	if !bytes.Contains(out, []byte("Usage:")) {
		t.Fatal("--help output missing Usage header")
	}
}
