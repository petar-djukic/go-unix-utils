// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd038-unlink R1.1–R1.3, R2.1–R2.4, R3.1–R3.3:
// compares stdout, stderr, exit codes via pkg/testutils.
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

// TestDiff runs differential tests for prd038-unlink comparing the Go
// binary against the GNU reference binary (gunlink).
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skipf("reference binary gunlink not in PATH: %v", err)
	}
	normBin := makeBinaryNormalizer(refBin)
	runSharedTests(t, goBin, refBin, normBin)
	runIsolatedTests(t, goBin, refBin, normBin)
}

// runSharedTests runs tests where both binaries can share the same WorkDir
// (error cases where no files are removed).
func runSharedTests(t *testing.T, goBin, refBin string, normBin testutils.NormalizeFunc) {
	t.Helper()
	tests := []testutils.DiffTest{
		// R2.1: zero arguments — missing operand error.
		{Name: "missing_operand",
			Normalize: []testutils.NormalizeFunc{normBin}},
		// R2.2: extra operand error with two arguments.
		{Name: "extra_operand", Args: []string{"a", "b"},
			Normalize: []testutils.NormalizeFunc{normBin}},
		// R2.3: non-existent file error.
		{Name: "nonexistent_file", Args: []string{"no_such_file"},
			Normalize: []testutils.NormalizeFunc{normBin}},
		// --version prints version info to stdout.
		{Name: "version", Args: []string{"--version"},
			Normalize: []testutils.NormalizeFunc{versionNormalizer, normBin}},
		// --help prints usage to stdout.
		{Name: "help", Args: []string{"--help"},
			Normalize: []testutils.NormalizeFunc{helpNormalizer, normBin}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// isolatedCase defines a test where each binary runs in its own temp dir
// to avoid cross-contamination from file removal side effects.
type isolatedCase struct {
	name  string
	args  []string
	setup func(t *testing.T, dir string)
	norm  []testutils.NormalizeFunc
	// checkRemoved is the relative path to verify no longer exists after run.
	checkRemoved string
}

// runIsolatedTests runs tests where each binary needs its own WorkDir
// because files are removed by the first binary.
func runIsolatedTests(t *testing.T, goBin, refBin string, normBin testutils.NormalizeFunc) {
	t.Helper()
	cases := []isolatedCase{
		// R1.1, R3.2: unlink a regular file.
		{name: "regular_file", args: []string{"target.txt"},
			checkRemoved: "target.txt",
			setup: func(t *testing.T, dir string) {
				writeFile(t, filepath.Join(dir, "target.txt"))
			}},
		// R3.2: unlink a symbolic link (removes the link, not the target).
		{name: "symlink", args: []string{"link"},
			checkRemoved: "link",
			setup: func(t *testing.T, dir string) {
				writeFile(t, filepath.Join(dir, "real.txt"))
				if err := os.Symlink("real.txt", filepath.Join(dir, "link")); err != nil {
					t.Fatalf("setup: creating symlink: %v", err)
				}
			}},
		// R2.4: directory argument produces an error.
		{name: "directory_error", args: []string{"somedir"},
			norm: []testutils.NormalizeFunc{normBin},
			setup: func(t *testing.T, dir string) {
				if err := os.Mkdir(filepath.Join(dir, "somedir"), 0o755); err != nil {
					t.Fatalf("setup: creating dir: %v", err)
				}
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
// stdout, stderr, and exit code. Optionally verifies file removal.
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
	// R3.3: verify the target file no longer exists after successful unlink.
	if tc.checkRemoved != "" {
		verifyRemoved(t, goDir, tc.checkRemoved)
	}
}

// verifyRemoved checks that a file no longer exists in the given directory.
func verifyRemoved(t *testing.T, dir, relPath string) {
	t.Helper()
	fullPath := filepath.Join(dir, relPath)
	if _, err := os.Lstat(fullPath); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, but it still exists", relPath)
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
// binary's full path and name with "unlink", then lowercases everything
// to handle strerror() capitalization differences.
func makeBinaryNormalizer(refBin string) testutils.NormalizeFunc {
	refDir := filepath.Dir(refBin)
	return func(data []byte) []byte {
		data = bytes.ReplaceAll(data, []byte(refBin), []byte("unlink"))
		if refDir != "" {
			data = bytes.ReplaceAll(data, []byte(refDir+"/unlink"), []byte("unlink"))
		}
		data = bytes.ReplaceAll(data, []byte("gunlink"), []byte("unlink"))
		return bytes.ToLower(data)
	}
}

// versionNormalizer reduces version output to just the program name
// on a single line. GNU outputs multi-line copyright info; we output
// a single line. Both start with "unlink" after binary name normalization.
func versionNormalizer(data []byte) []byte {
	if idx := bytes.IndexByte(data, ' '); idx >= 0 {
		return data[:idx]
	}
	return data
}

// helpNormalizer replaces all output with an empty byte slice so that
// --help tests compare only exit codes, not implementation-specific text.
func helpNormalizer(data []byte) []byte {
	return nil
}

// writeFile creates a small file at the given path.
func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: writing %s: %v", path, err)
	}
}
