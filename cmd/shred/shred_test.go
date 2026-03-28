// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/shred against GNU gshred.
// Covers prd099-shred R3.1-R3.3 (test scaffold, flag interactions, error cases).
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

// stderrNormalizer normalizes shred stderr output between Go and GNU.
// D3: strips binary paths, normalizes random hex patterns, and
// lowercases OS error messages for cross-platform comparison.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?shred|gshred`)
	randomHex := regexp.MustCompile(`\(([0-9a-f]{6})\)`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	// GNU prefixes file errors with "failed to open for writing: ".
	openPrefix := regexp.MustCompile(`failed to open for writing: `)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("shred"))
		b = randomHex.ReplaceAll(b, []byte("(RANDOM)"))
		b = tryHelp.ReplaceAll(b, nil)
		b = openPrefix.ReplaceAll(b, nil)
		b = bytes.ReplaceAll(b,
			[]byte("No such file or directory"),
			[]byte("no such file or directory"))
		b = bytes.ReplaceAll(b,
			[]byte("Permission denied"),
			[]byte("permission denied"))
		return b
	}
}

// createTestFile creates a file with known content in the given directory.
func createTestFile(t *testing.T, dir, name string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("create test file: %v", err)
	}
}

// mkTestDir creates a temp dir with a "target" file for shred tests.
func mkTestDir(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	createTestFile(t, dir, "target")
	return dir, "target"
}

// flagTests returns R3.2 test cases for non-destructive flag interactions.
// D1: compare exit codes and stderr patterns, not file contents.
// Tests with --remove are excluded because the reference binary deletes
// the shared WorkDir file before the Go binary runs.
func flagTests(t *testing.T, norm testutils.NormalizeFunc) []testutils.DiffTest {
	t.Helper()
	norms := []testutils.NormalizeFunc{norm}

	dir1, f1 := mkTestDir(t)
	dir2, f2 := mkTestDir(t)
	dir3, f3 := mkTestDir(t)
	dir4, f4 := mkTestDir(t)
	dir5, f5 := mkTestDir(t)
	dir6, f6 := mkTestDir(t)

	return []testutils.DiffTest{
		// R3.2: verbose output with default iterations.
		{
			Name:      "verbose_default",
			Args:      []string{"-v", f1},
			WorkDir:   dir1,
			Normalize: norms,
		},
		// R3.2: verbose with zero final pass.
		{
			Name:      "verbose_zero",
			Args:      []string{"-v", "-z", f2},
			WorkDir:   dir2,
			Normalize: norms,
		},
		// R3.2: single iteration with verbose.
		{
			Name:      "verbose_n1",
			Args:      []string{"-v", "-n", "1", f3},
			WorkDir:   dir3,
			Normalize: norms,
		},
		// R3.2: zero iterations with zero pass.
		{
			Name:      "n0_zero",
			Args:      []string{"-n", "0", "-z", f4},
			WorkDir:   dir4,
			Normalize: norms,
		},
		// R3.2: exact flag with verbose.
		{
			Name:      "exact_verbose",
			Args:      []string{"-v", "-x", f5},
			WorkDir:   dir5,
			Normalize: norms,
		},
		// R3.2: size flag limits bytes shredded.
		{
			Name:      "size_flag",
			Args:      []string{"-v", "--size=8", f6},
			WorkDir:   dir6,
			Normalize: norms,
		},
	}
}

// errorTests returns R3.3 test cases for error conditions.
func errorTests(norm testutils.NormalizeFunc) []testutils.DiffTest {
	norms := []testutils.NormalizeFunc{norm}
	return []testutils.DiffTest{
		// R3.3: nonexistent file.
		{
			Name:      "error_nonexistent",
			Args:      []string{"/nonexistent/path/file"},
			Normalize: norms,
		},
		// R3.3: missing file operand.
		{
			Name:      "error_no_operand",
			Args:      []string{},
			Normalize: norms,
		},
		// R3.3: invalid --iterations value.
		{
			Name:      "error_invalid_iterations",
			Args:      []string{"-n", "abc", "file"},
			Normalize: norms,
		},
		// R3.3: invalid option flag.
		{
			Name:      "error_invalid_option",
			Args:      []string{"-Q", "file"},
			Normalize: norms,
		},
	}
}

// TestDiff runs differential tests comparing Go shred against GNU gshred.
// R3.1: scaffold with BuildBinary, LookPath, and t.Skip.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshred")
	if err != nil {
		t.Skipf("reference binary gshred not in PATH: %v", err)
	}
	norm := stderrNormalizer()

	var tests []testutils.DiffTest
	tests = append(tests, flagTests(t, norm)...)
	tests = append(tests, errorTests(norm)...)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestRemove tests --remove flag independently since the reference binary
// deletes the file before the Go binary can access it in shared WorkDir.
// D1: compare only exit codes, not file contents.
func TestRemove(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshred")
	if err != nil {
		t.Skipf("reference binary gshred not in PATH: %v", err)
	}
	norm := stderrNormalizer()

	cases := []struct {
		name string
		args func(file string) []string
	}{
		{
			name: "remove_basic",
			args: func(f string) []string { return []string{"-u", f} },
		},
		{
			name: "remove_verbose",
			args: func(f string) []string { return []string{"-v", "-u", f} },
		},
		{
			name: "remove_zero_exact",
			args: func(f string) []string {
				return []string{"-u", "-z", "-x", f}
			},
		},
		{
			name: "remove_n1_vzu",
			args: func(f string) []string {
				return []string{"-vzu", "-n", "1", f}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runRemoveCase(t, goBin, refBin, norm, tc.args)
		})
	}
}

// runRemoveCase runs a --remove test with separate dirs for each binary.
func runRemoveCase(
	t *testing.T,
	goBin, refBin string,
	norm testutils.NormalizeFunc,
	argsFn func(string) []string,
) {
	t.Helper()
	fname := "target"

	// Run reference binary in its own directory.
	refDir := t.TempDir()
	createTestFile(t, refDir, fname)
	refExit := runShredBinary(t, refBin, refDir, argsFn(fname))

	// Run Go binary in its own directory.
	goDir := t.TempDir()
	createTestFile(t, goDir, fname)
	goExit := runShredBinary(t, goBin, goDir, argsFn(fname))

	if refExit != goExit {
		t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
	}
	// Verify file was removed by both binaries.
	refRemoved := fileRemoved(t, refDir, fname)
	goRemoved := fileRemoved(t, goDir, fname)
	if refRemoved != goRemoved {
		t.Errorf("file removed: ref=%v go=%v", refRemoved, goRemoved)
	}
	_ = norm // normalizer available for future stderr comparison
}

// runShredBinary runs a shred binary and returns the exit code.
func runShredBinary(
	t *testing.T, binary, dir string, args []string,
) int {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		t.Fatalf("failed to run %s: %v", binary, err)
	}
	return 0
}

// fileRemoved checks whether a file has been removed from the directory.
func fileRemoved(t *testing.T, dir, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, name))
	return os.IsNotExist(err)
}
