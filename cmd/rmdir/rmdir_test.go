// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/rmdir against GNU grmdir.
// Covers prd035-rmdir R1.1-R1.4 (basic removal, errors),
// R2.1-R2.2 (parent removal, stop ascending),
// R3.1 (--ignore-fail-on-non-empty).
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU grmdir and Go rmdir.
// Handles binary name differences and error message capitalization.
func stderrNormalizer() testutils.NormalizeFunc {
	binName := regexp.MustCompile(`/[^\s:]+/g?rmdir|grmdir`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	return func(b []byte) []byte {
		b = binName.ReplaceAll(b, []byte("rmdir"))
		b = tryHelp.ReplaceAll(b, nil)
		b = bytes.ToLower(b)
		return b
	}
}

// cmdResult holds captured output from a binary invocation.
type cmdResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runRmdir runs a binary with the given args in the given dir.
func runRmdir(t *testing.T, binary, dir string, args []string) cmdResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, runErr)
		}
	}
	return cmdResult{
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
	}
}

// compareResults asserts that ref and go binary outputs match.
func compareResults(t *testing.T, name string, ref, got cmdResult, norm testutils.NormalizeFunc) {
	t.Helper()
	refOut, refErr := ref.stdout, ref.stderr
	goOut, goErr := got.stdout, got.stderr
	if norm != nil {
		refOut = norm(refOut)
		refErr = norm(refErr)
		goOut = norm(goOut)
		goErr = norm(goErr)
	}
	if !bytes.Equal(refOut, goOut) {
		t.Errorf("%s: stdout mismatch\n  ref: %q\n  go:  %q", name, refOut, goOut)
	}
	if !bytes.Equal(refErr, goErr) {
		t.Errorf("%s: stderr mismatch\n  ref: %q\n  go:  %q", name, refErr, goErr)
	}
	if ref.exitCode != got.exitCode {
		t.Errorf("%s: exit code mismatch: ref=%d go=%d", name, ref.exitCode, got.exitCode)
	}
}

// TestDiffErrors tests error cases that don't mutate filesystem state.
func TestDiffErrors(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skipf("reference binary grmdir not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	// Pre-create a non-empty directory for the non-empty error test.
	nonEmptyDir := filepath.Join(t.TempDir(), "nonempty")
	mkdirWithFile(t, nonEmptyDir)

	tests := []testutils.DiffTest{
		// R1.4: nonexistent target.
		{
			Name:      "nonexistent_target",
			Args:      []string{"no_such_dir_xyz"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R1.3: non-empty directory.
		{
			Name:      "non_empty_dir",
			Args:      []string{nonEmptyDir},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// mkdirWithFile creates a directory with a file inside.
func mkdirWithFile(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	f := filepath.Join(dir, "file")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatalf("create file: %v", err)
	}
}

// assertDirRemoved verifies a directory no longer exists.
func assertDirRemoved(t *testing.T, path, label string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("%s: directory %s still exists after rmdir", label, path)
	}
}

// TestRmdirSingleEmpty verifies removal of a single empty directory.
// R1.1: remove one empty directory.
func TestRmdirSingleEmpty(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skipf("reference binary grmdir not in PATH: %v", err)
	}

	norm := stderrNormalizer()

	// Run reference binary.
	refDir := t.TempDir()
	refTarget := filepath.Join(refDir, "emptydir")
	os.Mkdir(refTarget, 0o755) //nolint:errcheck
	refRes := runRmdir(t, refBin, refDir, []string{"emptydir"})
	assertDirRemoved(t, refTarget, "ref")

	// Run Go binary.
	goDir := t.TempDir()
	goTarget := filepath.Join(goDir, "emptydir")
	os.Mkdir(goTarget, 0o755) //nolint:errcheck
	goRes := runRmdir(t, goBin, goDir, []string{"emptydir"})
	assertDirRemoved(t, goTarget, "go")

	compareResults(t, "single_empty", refRes, goRes, norm)
}

// TestRmdirMultiple verifies removal of multiple directories.
// R1.2: multiple directories processed left to right.
func TestRmdirMultiple(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skipf("reference binary grmdir not in PATH: %v", err)
	}

	norm := stderrNormalizer()

	// Run reference binary.
	refDir := t.TempDir()
	os.Mkdir(filepath.Join(refDir, "a"), 0o755) //nolint:errcheck
	os.Mkdir(filepath.Join(refDir, "b"), 0o755) //nolint:errcheck
	refRes := runRmdir(t, refBin, refDir, []string{"a", "b"})
	assertDirRemoved(t, filepath.Join(refDir, "a"), "ref-a")
	assertDirRemoved(t, filepath.Join(refDir, "b"), "ref-b")

	// Run Go binary.
	goDir := t.TempDir()
	os.Mkdir(filepath.Join(goDir, "a"), 0o755) //nolint:errcheck
	os.Mkdir(filepath.Join(goDir, "b"), 0o755) //nolint:errcheck
	goRes := runRmdir(t, goBin, goDir, []string{"a", "b"})
	assertDirRemoved(t, filepath.Join(goDir, "a"), "go-a")
	assertDirRemoved(t, filepath.Join(goDir, "b"), "go-b")

	compareResults(t, "multiple", refRes, goRes, norm)
}

// TestRmdirParents verifies -p removes directory and empty ancestors.
// R2.1: removes target and ancestors.
// R2.2: stops on non-empty parent.
func TestRmdirParents(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skipf("reference binary grmdir not in PATH: %v", err)
	}

	norm := stderrNormalizer()

	// Run reference binary.
	refDir := t.TempDir()
	os.MkdirAll(filepath.Join(refDir, "a", "b", "c"), 0o755) //nolint:errcheck
	refRes := runRmdir(t, refBin, refDir, []string{"-p", "a/b/c"})
	assertDirRemoved(t, filepath.Join(refDir, "a"), "ref-a")

	// Run Go binary.
	goDir := t.TempDir()
	os.MkdirAll(filepath.Join(goDir, "a", "b", "c"), 0o755) //nolint:errcheck
	goRes := runRmdir(t, goBin, goDir, []string{"-p", "a/b/c"})
	assertDirRemoved(t, filepath.Join(goDir, "a"), "go-a")

	compareResults(t, "parents", refRes, goRes, norm)
}

// TestRmdirParentsStopNonEmpty verifies -p stops at non-empty ancestor.
// R2.2: stops ascending when parent is not empty.
func TestRmdirParentsStopNonEmpty(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skipf("reference binary grmdir not in PATH: %v", err)
	}

	norm := stderrNormalizer()

	// Run reference binary: a/b/c where a has a sibling file.
	refDir := t.TempDir()
	os.MkdirAll(filepath.Join(refDir, "a", "b", "c"), 0o755)        //nolint:errcheck
	os.WriteFile(filepath.Join(refDir, "a", "keep"), []byte("x"), 0o644) //nolint:errcheck
	refRes := runRmdir(t, refBin, refDir, []string{"-p", "a/b/c"})
	// "a" should still exist because it's not empty.
	if _, err := os.Stat(filepath.Join(refDir, "a")); os.IsNotExist(err) {
		t.Error("ref: a should still exist")
	}
	assertDirRemoved(t, filepath.Join(refDir, "a", "b"), "ref-b")

	// Run Go binary.
	goDir := t.TempDir()
	os.MkdirAll(filepath.Join(goDir, "a", "b", "c"), 0o755)        //nolint:errcheck
	os.WriteFile(filepath.Join(goDir, "a", "keep"), []byte("x"), 0o644) //nolint:errcheck
	goRes := runRmdir(t, goBin, goDir, []string{"-p", "a/b/c"})
	if _, err := os.Stat(filepath.Join(goDir, "a")); os.IsNotExist(err) {
		t.Error("go: a should still exist")
	}
	assertDirRemoved(t, filepath.Join(goDir, "a", "b"), "go-b")

	compareResults(t, "parents_stop", refRes, goRes, norm)
}

// TestRmdirIgnoreNonEmpty verifies --ignore-fail-on-non-empty.
// R3.1: suppresses non-empty error and exit code.
func TestRmdirIgnoreNonEmpty(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skipf("reference binary grmdir not in PATH: %v", err)
	}

	norm := stderrNormalizer()

	// Run reference binary on a non-empty directory.
	refDir := t.TempDir()
	mkdirWithFile(t, filepath.Join(refDir, "nonempty"))
	refRes := runRmdir(t, refBin, refDir,
		[]string{"--ignore-fail-on-non-empty", "nonempty"})

	// Run Go binary on a non-empty directory.
	goDir := t.TempDir()
	mkdirWithFile(t, filepath.Join(goDir, "nonempty"))
	goRes := runRmdir(t, goBin, goDir,
		[]string{"--ignore-fail-on-non-empty", "nonempty"})

	compareResults(t, "ignore_non_empty", refRes, goRes, norm)
}
