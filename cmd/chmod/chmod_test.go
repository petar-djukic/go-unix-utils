// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/chmod against gchmod (GNU coreutils).
// Implements srd089 R1.1-R1.4, R2.1-R2.4.
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gchmod"
const execTimeout = 30 * time.Second

// makeNormalizer creates a NormalizeFunc that replaces binary-specific names
// and normalizes syscall error message capitalization.
func makeNormalizer(refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte(programName))
		b = bytes.ReplaceAll(b, []byte(refBinName), []byte(programName))
		b = normalizeSyscallErrors(b)
		return b
	}
}

// normalizeSyscallErrors lowercases known syscall error messages that
// differ in case between C strerror() and Go syscall.Errno.Error().
func normalizeSyscallErrors(b []byte) []byte {
	replacements := []struct{ from, to string }{
		{"No such file or directory", "no such file or directory"},
		{"Not a directory", "not a directory"},
		{"Permission denied", "permission denied"},
		{"Operation not permitted", "operation not permitted"},
	}
	for _, r := range replacements {
		b = bytes.ReplaceAll(b, []byte(r.from), []byte(r.to))
	}
	return b
}

// TestDiff runs differential tests comparing cmd/chmod against gchmod.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}
	norm := makeNormalizer(refBin)

	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		runErrorTests(t, goBin, refBin, norm)
	})
	t.Run("octal", func(t *testing.T) {
		t.Parallel()
		runOctalTests(t, goBin, refBin, norm)
	})
	t.Run("symbolic", func(t *testing.T) {
		t.Parallel()
		runSymbolicTests(t, goBin, refBin, norm)
	})
	t.Run("multiple_files", func(t *testing.T) {
		t.Parallel()
		runMultipleFileTests(t, goBin, refBin, norm)
	})
	t.Run("error_continue", func(t *testing.T) {
		t.Parallel()
		runErrorContinueTests(t, goBin, refBin, norm)
	})
	// R2.1-R2.4 test groups
	t.Run("recursive", func(t *testing.T) {
		t.Parallel()
		runRecursiveTests(t, goBin, refBin, norm)
	})
	t.Run("verbose", func(t *testing.T) {
		t.Parallel()
		runVerboseTests(t, goBin, refBin, norm)
	})
	t.Run("changes", func(t *testing.T) {
		t.Parallel()
		runChangesTests(t, goBin, refBin, norm)
	})
	t.Run("silent", func(t *testing.T) {
		t.Parallel()
		runSilentTests(t, goBin, refBin, norm)
	})
}

// runErrorTests tests error cases where no file modification occurs.
func runErrorTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	norms := []testutils.NormalizeFunc{norm}
	tests := []testutils.DiffTest{
		{
			Name:      "nonexistent_file",
			Args:      []string{"755", "nonexistent"},
			ExitCode:  1,
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// chmodCase defines an isolated chmod test case.
type chmodCase struct {
	name     string
	args     []string
	initMode os.FileMode
}

// runOctalTests tests R1.1: octal mode application.
func runOctalTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	cases := []chmodCase{
		{name: "755_from_644", args: []string{"755", "file"}, initMode: 0o644},
		{name: "644_from_755", args: []string{"644", "file"}, initMode: 0o755},
		{name: "0755_leading_zero", args: []string{"0755", "file"}, initMode: 0o644},
		{name: "600_from_777", args: []string{"600", "file"}, initMode: 0o777},
		{name: "000_clear_all", args: []string{"000", "file"}, initMode: 0o755},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runIsolatedChmod(t, goBin, refBin, norm, tc)
		})
	}
}

// runSymbolicTests tests R1.2: symbolic mode application.
func runSymbolicTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	cases := []chmodCase{
		{name: "u_plus_x", args: []string{"u+x", "file"}, initMode: 0o644},
		{name: "go_minus_w", args: []string{"go-w", "file"}, initMode: 0o666},
		{name: "a_equals_rw", args: []string{"a=rw", "file"}, initMode: 0o755},
		{name: "u_rwx_go_rx", args: []string{"u=rwx,go=rx", "file"}, initMode: 0o000},
		{name: "plus_x", args: []string{"+x", "file"}, initMode: 0o644},
		{name: "o_plus_r", args: []string{"o+r", "file"}, initMode: 0o640},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runIsolatedChmod(t, goBin, refBin, norm, tc)
		})
	}
}

// runMultipleFileTests tests R1.3: processing multiple FILE arguments.
func runMultipleFileTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	t.Run("two_files_octal", func(t *testing.T) {
		t.Parallel()
		refDir := t.TempDir()
		goDir := t.TempDir()

		setupFile(t, refDir, "file1", 0o644)
		setupFile(t, refDir, "file2", 0o600)
		setupFile(t, goDir, "file1", 0o644)
		setupFile(t, goDir, "file2", 0o600)

		args := []string{"755", "file1", "file2"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)

		compareOutputs(t, norm, refRes, goRes)
		compareFilePerm(t, refDir, goDir, "file1")
		compareFilePerm(t, refDir, goDir, "file2")
	})
}

// runErrorContinueTests tests R1.4: continue processing after error.
func runErrorContinueTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	t.Run("nonexistent_then_real", func(t *testing.T) {
		t.Parallel()
		refDir := t.TempDir()
		goDir := t.TempDir()

		setupFile(t, refDir, "realfile", 0o644)
		setupFile(t, goDir, "realfile", 0o644)

		args := []string{"755", "nonexistent", "realfile"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)

		compareOutputs(t, norm, refRes, goRes)
		compareFilePerm(t, refDir, goDir, "realfile")
	})
}

// runRecursiveTests tests R2.1: -R recursive mode changes.
func runRecursiveTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	t.Run("recursive_octal_755", func(t *testing.T) {
		t.Parallel()
		refDir := t.TempDir()
		goDir := t.TempDir()

		setupTree(t, refDir, 0o644)
		setupTree(t, goDir, 0o644)

		args := []string{"-R", "755", "testdir"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)

		compareOutputs(t, norm, refRes, goRes)
		compareFilePerm(t, refDir, goDir, filepath.Join("testdir", "file1"))
		compareFilePerm(t, refDir, goDir, filepath.Join("testdir", "sub"))
		compareFilePerm(t, refDir, goDir, filepath.Join("testdir", "sub", "file2"))
	})
	t.Run("recursive_symbolic_a_plus_x", func(t *testing.T) {
		t.Parallel()
		refDir := t.TempDir()
		goDir := t.TempDir()

		setupTree(t, refDir, 0o644)
		setupTree(t, goDir, 0o644)

		args := []string{"-R", "a+x", "testdir"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)

		compareOutputs(t, norm, refRes, goRes)
		compareFilePerm(t, refDir, goDir, filepath.Join("testdir", "file1"))
		compareFilePerm(t, refDir, goDir, filepath.Join("testdir", "sub"))
		compareFilePerm(t, refDir, goDir, filepath.Join("testdir", "sub", "file2"))
	})
}

// runVerboseTests tests R2.2: -v verbose diagnostic output.
func runVerboseTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	t.Run("verbose_mode_change", func(t *testing.T) {
		t.Parallel()
		refDir := t.TempDir()
		goDir := t.TempDir()

		setupFile(t, refDir, "file", 0o644)
		setupFile(t, goDir, "file", 0o644)

		args := []string{"-v", "755", "file"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)

		compareOutputs(t, norm, refRes, goRes)
	})
	t.Run("verbose_no_change", func(t *testing.T) {
		t.Parallel()
		refDir := t.TempDir()
		goDir := t.TempDir()

		setupFile(t, refDir, "file", 0o644)
		setupFile(t, goDir, "file", 0o644)

		args := []string{"-v", "644", "file"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)

		compareOutputs(t, norm, refRes, goRes)
	})
}

// runChangesTests tests R2.3: -c changes-only diagnostic output.
func runChangesTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	t.Run("changes_mode_changed", func(t *testing.T) {
		t.Parallel()
		refDir := t.TempDir()
		goDir := t.TempDir()

		setupFile(t, refDir, "file", 0o644)
		setupFile(t, goDir, "file", 0o644)

		args := []string{"-c", "755", "file"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)

		compareOutputs(t, norm, refRes, goRes)
	})
	t.Run("changes_no_change", func(t *testing.T) {
		t.Parallel()
		refDir := t.TempDir()
		goDir := t.TempDir()

		setupFile(t, refDir, "file", 0o644)
		setupFile(t, goDir, "file", 0o644)

		args := []string{"-c", "644", "file"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)

		compareOutputs(t, norm, refRes, goRes)
	})
}

// runSilentTests tests R2.4: -f silent/quiet error suppression.
func runSilentTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	t.Run("silent_nonexistent", func(t *testing.T) {
		t.Parallel()
		refDir := t.TempDir()
		goDir := t.TempDir()

		args := []string{"-f", "755", "nonexistent"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)

		compareOutputs(t, norm, refRes, goRes)
	})
}

// setupTree creates a directory tree for recursive tests:
// root/testdir/file1, root/testdir/sub/file2.
func setupTree(t *testing.T, root string, mode os.FileMode) {
	t.Helper()
	dir := filepath.Join(root, "testdir")
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("setup MkdirAll: %v", err)
	}
	setupFile(t, dir, "file1", mode)
	setupFile(t, sub, "file2", mode)
	if err := os.Chmod(sub, mode); err != nil {
		t.Fatalf("setup Chmod sub: %v", err)
	}
}

// runIsolatedChmod runs a single chmod test in isolated directories.
func runIsolatedChmod(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc, tc chmodCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	setupFile(t, refDir, "file", tc.initMode)
	setupFile(t, goDir, "file", tc.initMode)

	refRes := runBin(t, refBin, tc.args, refDir)
	goRes := runBin(t, goBin, tc.args, goDir)

	compareOutputs(t, norm, refRes, goRes)
	compareFilePerm(t, refDir, goDir, "file")
}

// setupFile creates a file with the given permissions.
func setupFile(t *testing.T, dir, name string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("test"), 0o666); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("setup Chmod: %v", err)
	}
}

// compareFilePerm checks that a file has the same permissions in both dirs.
func compareFilePerm(t *testing.T, refDir, goDir, name string) {
	t.Helper()
	refInfo, err := os.Lstat(filepath.Join(refDir, name))
	if err != nil {
		return // file doesn't exist in ref, error caught by output comparison
	}
	goInfo, err := os.Lstat(filepath.Join(goDir, name))
	if err != nil {
		t.Errorf("file %s: exists in ref but not in go dir", name)
		return
	}
	refPerm := refInfo.Mode().Perm()
	goPerm := goInfo.Mode().Perm()
	if refPerm != goPerm {
		t.Errorf("file %s: perm mismatch ref=%04o go=%04o",
			name, refPerm, goPerm)
	}
}

// binResult holds captured output from a single binary execution.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runBin executes a binary in workDir and captures stdout, stderr, exit code.
func runBin(t *testing.T, bin string, args []string, workDir string) binResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Dir = workDir
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)

	return extractResult(t, cmd, ctx, &outBuf, &errBuf)
}

// extractResult runs the command and returns the captured result.
func extractResult(t *testing.T, cmd *exec.Cmd, ctx context.Context, outBuf, errBuf *bytes.Buffer) binResult {
	t.Helper()
	err := cmd.Run()
	if err == nil {
		return binResult{stdout: outBuf.Bytes(), stderr: errBuf.Bytes()}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return binResult{
			stdout:   outBuf.Bytes(),
			stderr:   errBuf.Bytes(),
			exitCode: exitErr.ExitCode(),
		}
	}
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("%s timed out after %v", cmd.Path, execTimeout)
	}
	t.Fatalf("%s failed: %v", cmd.Path, err)
	return binResult{} // unreachable
}

// compareOutputs compares stdout, stderr, and exit code between ref and go.
func compareOutputs(t *testing.T, norm testutils.NormalizeFunc, ref, got binResult) {
	t.Helper()
	refOut := norm(ref.stdout)
	gotOut := norm(got.stdout)
	refErr := norm(ref.stderr)
	gotErr := norm(got.stderr)

	if !bytes.Equal(refOut, gotOut) {
		t.Errorf("stdout mismatch\nref: %q\ngot: %q", refOut, gotOut)
	}
	if !bytes.Equal(refErr, gotErr) {
		t.Errorf("stderr mismatch\nref: %q\ngot: %q", refErr, gotErr)
	}
	if ref.exitCode != got.exitCode {
		t.Errorf("exit code mismatch: ref=%d got=%d", ref.exitCode, got.exitCode)
	}
}
