// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mkdir against gmkdir (GNU coreutils).
// Implements srd034 R4.1 (compare stdout/stderr/exit codes),
// R4.2 (test coverage), R4.3 (permission verification).
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

const refBinName = "gmkdir"
const execTimeout = 30 * time.Second

// makeNormalizer creates a NormalizeFunc that normalizes binary names and
// known syscall error message capitalization differences between GNU and Go.
func makeNormalizer(refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		// Replace full ref binary path (e.g. /opt/homebrew/bin/mkdir).
		b = bytes.ReplaceAll(b, []byte(refBin), []byte(programName))
		// Replace gmkdir prefix (in case path wasn't absolute).
		b = bytes.ReplaceAll(b, []byte(refBinName), []byte(programName))
		// Normalize syscall error message capitalization.
		b = normalizeSyscallErrors(b)
		return b
	}
}

// normalizeSyscallErrors lowercases known syscall error messages that
// differ in case between C strerror() and Go syscall.Errno.Error().
func normalizeSyscallErrors(b []byte) []byte {
	replacements := []struct{ from, to string }{
		{"File exists", "file exists"},
		{"No such file or directory", "no such file or directory"},
		{"Not a directory", "not a directory"},
		{"Permission denied", "permission denied"},
	}
	for _, r := range replacements {
		b = bytes.ReplaceAll(b, []byte(r.from), []byte(r.to))
	}
	return b
}

// TestDiff runs differential tests comparing cmd/mkdir against gmkdir.
// R4.1: compare stdout, stderr, exit codes.
// R4.2: covers single, multiple, -p, -m, -v, and error cases.
// R4.3: verifies permission bits match.
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
	t.Run("creation", func(t *testing.T) {
		t.Parallel()
		runCreationTests(t, goBin, refBin, norm)
	})
}

// runErrorTests uses RunDiffTests for error cases where both binaries
// see the same filesystem state and neither mutates it.
func runErrorTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "existing"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	norms := []testutils.NormalizeFunc{norm}
	tests := []testutils.DiffTest{
		{
			Name: "missing_operand", Args: []string{},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "existing_dir_no_p", Args: []string{"existing"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "existing_dir_with_p", Args: []string{"-p", "existing"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "missing_parent", Args: []string{"no/such/dir"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// isolatedCase defines a creation test that runs each binary in its own dir.
type isolatedCase struct {
	name      string
	args      []string
	checkDirs []string // relative paths whose permissions to compare
}

// runCreationTests runs tests where each binary operates in an isolated
// temp directory so filesystem mutations do not interfere.
func runCreationTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	cases := creationCases()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareIsolated(t, goBin, refBin, norm, tc)
		})
	}
}

// creationCases returns the table of isolated creation test cases.
// R4.2: single, multiple, -p nested, -m octal, -v, and combinations.
func creationCases() []isolatedCase {
	return []isolatedCase{
		{name: "single_dir", args: []string{"newdir"}, checkDirs: []string{"newdir"}},
		{name: "multiple_dirs", args: []string{"d1", "d2", "d3"}, checkDirs: []string{"d1", "d2", "d3"}},
		{name: "parents_nested", args: []string{"-p", "a/b/c"}, checkDirs: []string{"a", "a/b", "a/b/c"}},
		{name: "mode_0755", args: []string{"-m", "0755", "mdir"}, checkDirs: []string{"mdir"}},
		{name: "mode_0700", args: []string{"-m", "0700", "rdir"}, checkDirs: []string{"rdir"}},
		{name: "verbose", args: []string{"-v", "vdir"}, checkDirs: []string{"vdir"}},
		{name: "verbose_parents", args: []string{"-v", "-p", "x/y/z"}, checkDirs: []string{"x", "x/y", "x/y/z"}},
		{name: "parents_mode", args: []string{"-p", "-m", "0700", "p/q/r"}, checkDirs: []string{"p", "p/q", "p/q/r"}},
	}
}

// binResult holds captured output from a single binary execution.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// compareIsolated runs both binaries in separate temp dirs and compares
// stdout, stderr, exit code, and directory permissions.
func compareIsolated(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc, tc isolatedCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	refRes := runBin(t, refBin, tc.args, refDir)
	goRes := runBin(t, goBin, tc.args, goDir)

	compareOutputs(t, norm, refRes, goRes)
	compareDirPerms(t, refDir, goDir, tc.checkDirs)
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
		return binResult{stdout: outBuf.Bytes(), stderr: errBuf.Bytes(), exitCode: exitErr.ExitCode()}
	}
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("%s timed out after %v", cmd.Path, execTimeout)
	}
	t.Fatalf("%s failed: %v", cmd.Path, err)
	return binResult{} // unreachable
}

// compareOutputs compares stdout, stderr, and exit code between ref and go.
// Normalizes binary names and known error message differences.
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

// compareDirPerms verifies directory permissions match between ref and go.
// R4.3: checks all dirs created by -m and -p combinations.
func compareDirPerms(t *testing.T, refDir, goDir string, checkDirs []string) {
	t.Helper()
	for _, rel := range checkDirs {
		comparePerm(t, rel, filepath.Join(refDir, rel), filepath.Join(goDir, rel))
	}
}

// comparePerm checks that a single directory exists in both trees with
// matching permission bits.
func comparePerm(t *testing.T, name, refPath, goPath string) {
	t.Helper()
	refInfo, err := os.Stat(refPath)
	if err != nil {
		t.Errorf("ref dir %s missing: %v", name, err)
		return
	}
	goInfo, err := os.Stat(goPath)
	if err != nil {
		t.Errorf("go dir %s missing: %v", name, err)
		return
	}
	if refInfo.Mode().Perm() != goInfo.Mode().Perm() {
		t.Errorf("perm mismatch %s: ref=%o got=%o", name, refInfo.Mode().Perm(), goInfo.Mode().Perm())
	}
}

