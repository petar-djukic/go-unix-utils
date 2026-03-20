// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd035-rmdir R1.1–R1.4, R2.1–R2.3, R3.1.
// R4.1: compares stdout, stderr, exit codes via pkg/testutils.
package main_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests for prd035-rmdir comparing the Go
// binary against the GNU reference binary (grmdir).
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skipf("reference binary grmdir not in PATH: %v", err)
	}
	normBin := makeBinaryNormalizer(refBin)
	runSharedTests(t, goBin, refBin, normBin)
	runIsolatedTests(t, goBin, refBin, normBin)
}

// runSharedTests runs tests where both binaries can share the same WorkDir
// without diverging (error cases where no directories are removed).
func runSharedTests(t *testing.T, goBin, refBin string, normBin testutils.NormalizeFunc) {
	t.Helper()
	// Prepare a non-empty directory for error and suppression tests.
	nonEmptyBase := t.TempDir()
	mkdirAll(t, nonEmptyBase, "nonempty")
	writeFile(t, filepath.Join(nonEmptyBase, "nonempty", "file.txt"))

	tests := []testutils.DiffTest{
		// R1.4: non-existent directory error.
		{Name: "nonexistent_error", Args: []string{"no_such_dir"},
			Normalize: []testutils.NormalizeFunc{normBin}},
		// R1.3: non-empty directory error.
		{Name: "nonempty_error", Args: []string{"nonempty"},
			WorkDir:   nonEmptyBase,
			Normalize: []testutils.NormalizeFunc{normBin}},
		// R3.1: --ignore-fail-on-non-empty suppresses non-empty error.
		{Name: "ignore_nonempty", Args: []string{"--ignore-fail-on-non-empty", "nonempty"},
			WorkDir:   nonEmptyBase,
			Normalize: []testutils.NormalizeFunc{normBin}},
		// Missing operand.
		{Name: "missing_operand",
			Normalize: []testutils.NormalizeFunc{normBin}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// isolatedCase defines a test where each binary runs in its own temp dir
// to avoid cross-contamination from directory removal side effects.
type isolatedCase struct {
	name  string
	args  []string
	setup func(t *testing.T, dir string)
	norm  []testutils.NormalizeFunc
}

// runIsolatedTests runs tests where each binary needs its own WorkDir
// because directories are removed by the first binary.
func runIsolatedTests(t *testing.T, goBin, refBin string, normBin testutils.NormalizeFunc) {
	t.Helper()
	cases := []isolatedCase{
		// R1.1: remove a single empty directory.
		{name: "single_empty", args: []string{"emptydir"},
			setup: func(t *testing.T, dir string) {
				mkdirAll(t, dir, "emptydir")
			}},
		// R1.2: remove multiple empty directories independently.
		{name: "multiple_empty", args: []string{"d1", "d2"},
			setup: func(t *testing.T, dir string) {
				mkdirAll(t, dir, "d1")
				mkdirAll(t, dir, "d2")
			}},
		// R2.1: -p removes target and all empty parents.
		{name: "parents_nested", args: []string{"-p", "a/b/c"},
			setup: func(t *testing.T, dir string) {
				mkdirAll(t, dir, "a/b/c")
			}},
		// R2.2: -p stops when a parent is not empty.
		{name: "parents_stop_nonempty", args: []string{"-p", "a/b/c"},
			norm: []testutils.NormalizeFunc{normBin},
			setup: func(t *testing.T, dir string) {
				mkdirAll(t, dir, "a/b/c")
				writeFile(t, filepath.Join(dir, "a", "sibling.txt"))
			}},
		// R2.3: -p with multiple arguments processed independently.
		{name: "parents_multiple", args: []string{"-p", "x/y", "p/q"},
			setup: func(t *testing.T, dir string) {
				mkdirAll(t, dir, "x/y")
				mkdirAll(t, dir, "p/q")
			}},
		// R3.1 + R2.1: -p --ignore-fail-on-non-empty suppresses parent error.
		{name: "parents_ignore_nonempty",
			args: []string{"-p", "--ignore-fail-on-non-empty", "a/b/c"},
			setup: func(t *testing.T, dir string) {
				mkdirAll(t, dir, "a/b/c")
				writeFile(t, filepath.Join(dir, "a", "sibling.txt"))
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareIsolated(t, goBin, refBin, tc)
		})
	}
}

// compareIsolated runs both binaries in separate temp dirs and compares
// stdout, stderr, and exit code.
func compareIsolated(t *testing.T, goBin, refBin string, tc isolatedCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	if tc.setup != nil {
		tc.setup(t, refDir)
		tc.setup(t, goDir)
	}
	refOut, refErr, refCode := execBinary(t, refBin, tc.args, refDir)
	goOut, goErr, goCode := execBinary(t, goBin, tc.args, goDir)
	refOut, goOut, refErr, goErr = applyNorm(tc.norm, refOut, goOut, refErr, goErr)
	if !bytes.Equal(refOut, goOut) || !bytes.Equal(refErr, goErr) || refCode != goCode {
		t.Fatalf("divergence\nargs:       %v\n"+
			"ref stdout: %s\ngo  stdout: %s\n"+
			"ref stderr: %s\ngo  stderr: %s\n"+
			"ref exit:   %d\ngo  exit:   %d",
			tc.args, refOut, goOut, refErr, goErr, refCode, goCode)
	}
}

// applyNorm applies normalizers to ref and go stdout/stderr pairs.
func applyNorm(norm []testutils.NormalizeFunc, refOut, goOut, refErr, goErr []byte) ([]byte, []byte, []byte, []byte) {
	for _, n := range norm {
		refOut = n(refOut)
		goOut = n(goOut)
		refErr = n(refErr)
		goErr = n(goErr)
	}
	return refOut, goOut, refErr, goErr
}

// execBinary runs a binary with args in dir, returning stdout, stderr,
// and exit code.
func execBinary(t *testing.T, bin string, args []string, dir string) ([]byte, []byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = buildTestEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", bin)
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return stdout.Bytes(), stderr.Bytes(), exitErr.ExitCode()
		}
		t.Fatalf("binary %s failed to execute: %v", bin, err)
	}
	return stdout.Bytes(), stderr.Bytes(), 0
}

// buildTestEnv returns the process environment with LC_ALL=C set.
func buildTestEnv() []string {
	env := os.Environ()
	prefix := "LC_ALL="
	for i, e := range env {
		if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
			env[i] = "LC_ALL=C"
			return env
		}
	}
	return append(env, "LC_ALL=C")
}

// makeBinaryNormalizer returns a normalizer that replaces the reference
// binary's full path and name with "rmdir", then lowercases everything
// to handle strerror() capitalization differences.
func makeBinaryNormalizer(refBin string) testutils.NormalizeFunc {
	refDir := filepath.Dir(refBin)
	return func(data []byte) []byte {
		data = bytes.ReplaceAll(data, []byte(refBin), []byte("rmdir"))
		if refDir != "" {
			data = bytes.ReplaceAll(data, []byte(refDir+"/rmdir"), []byte("rmdir"))
		}
		data = bytes.ReplaceAll(data, []byte("grmdir"), []byte("rmdir"))
		return bytes.ToLower(data)
	}
}

// mkdirAll creates a nested directory structure inside base.
func mkdirAll(t *testing.T, base, subpath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(base, subpath), 0o755); err != nil {
		t.Fatalf("setup: creating %s: %v", subpath, err)
	}
}

// writeFile creates a small file at the given path.
func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: writing %s: %v", path, err)
	}
}
