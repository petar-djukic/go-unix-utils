// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/chgrp comparing against gchgrp (GNU coreutils).
// Covers prd090-chgrp R3.2 (exit codes), R3.3 (SIGPIPE handling).
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// cmdResult holds captured output from a binary invocation.
type cmdResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// stderrNormalizer normalizes error messages between GNU gchgrp and Go chgrp.
func stderrNormalizer() testutils.NormalizeFunc {
	binName := regexp.MustCompile(`/[^\s:]+/g?chgrp|gchgrp`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	goErrWrap := regexp.MustCompile(`(: )(stat|lstat|chown|lchown) [^:]+: `)
	return func(b []byte) []byte {
		b = binName.ReplaceAll(b, []byte("chgrp"))
		b = tryHelp.ReplaceAll(b, nil)
		b = goErrWrap.ReplaceAll(b, []byte("$1"))
		b = bytes.ToLower(b)
		return b
	}
}

// stdoutNormalizer normalizes verbose output paths between temp dirs.
func stdoutNormalizer(dir string) testutils.NormalizeFunc {
	re := regexp.MustCompile(regexp.QuoteMeta(dir))
	return func(b []byte) []byte {
		return re.ReplaceAll(b, []byte("/TMPDIR"))
	}
}

// runBin runs a binary with args in the given working directory.
func runBin(t *testing.T, binary string, args []string, dir string) cmdResult {
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
func compareResults(t *testing.T, name string, ref, got cmdResult) {
	t.Helper()
	norm := stderrNormalizer()
	if !bytes.Equal(ref.stdout, got.stdout) {
		t.Errorf("[%s] stdout mismatch\n  ref: %q\n  go:  %q",
			name, ref.stdout, got.stdout)
	}
	refErr := norm(ref.stderr)
	goErr := norm(got.stderr)
	if !bytes.Equal(refErr, goErr) {
		t.Errorf("[%s] stderr mismatch\n  ref: %q\n  go:  %q",
			name, refErr, goErr)
	}
	if ref.exitCode != got.exitCode {
		t.Errorf("[%s] exit code mismatch: ref=%d go=%d",
			name, ref.exitCode, got.exitCode)
	}
}

// writeFile creates a file with given content and mode.
func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

// lookupBinaries builds the Go binary and finds gchgrp.
func lookupBinaries(t *testing.T) (string, string) {
	t.Helper()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gchgrp")
	if err != nil {
		t.Skipf("reference binary gchgrp not in PATH: %v", err)
	}
	return goBin, refBin
}

// currentGroupName returns the group name of the current process's GID.
func currentGroupName(t *testing.T) string {
	t.Helper()
	gid := os.Getgid()
	g, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		t.Skipf("cannot resolve current GID %d: %v", gid, err)
	}
	return g.Name
}

// --- R3.2: Exit code differential tests ---

// TestDiffExitSuccessBasic tests exit 0 when all files succeed.
// R3.2: must exit 0 when all files are processed successfully.
func TestDiffExitSuccessBasic(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	groupName := currentGroupName(t)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "data", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "data", 0o644)

	refRes := runBin(t, refBin, []string{groupName, "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{groupName, "f.txt"}, goDir)
	compareResults(t, "exit_success_basic", refRes, goRes)

	if goRes.exitCode != 0 {
		t.Errorf("expected exit 0, got %d", goRes.exitCode)
	}
}

// TestDiffExitSuccessMultipleFiles tests exit 0 with multiple files.
// R3.2: all files processed → exit 0.
func TestDiffExitSuccessMultipleFiles(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	groupName := currentGroupName(t)

	refDir := t.TempDir()
	goDir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		writeFile(t, filepath.Join(refDir, name), "x", 0o644)
		writeFile(t, filepath.Join(goDir, name), "x", 0o644)
	}

	refRes := runBin(t, refBin,
		[]string{groupName, "a.txt", "b.txt", "c.txt"}, refDir)
	goRes := runBin(t, goBin,
		[]string{groupName, "a.txt", "b.txt", "c.txt"}, goDir)
	compareResults(t, "exit_success_multi", refRes, goRes)
}

// TestDiffExitErrorNonexistentFile tests exit 1 on nonexistent file.
// R3.2: must exit 1 on any error.
func TestDiffExitErrorNonexistentFile(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	groupName := currentGroupName(t)
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		{
			Name:      "nonexistent_file",
			Args:      []string{groupName, "/no/such/file/chgrp_test"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffExitErrorInvalidGroup tests exit 1 with invalid group name.
// R3.2: invalid group produces exit 1.
func TestDiffExitErrorInvalidGroup(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	errNorm := stderrNormalizer()

	tmpFile := filepath.Join(t.TempDir(), "f.txt")
	writeFile(t, tmpFile, "data", 0o644)

	tests := []testutils.DiffTest{
		{
			Name:      "invalid_group_name",
			Args:      []string{"no_such_group_xyz_test_99", tmpFile},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffExitErrorMissingOperand tests exit 1 with no arguments.
// R3.2: missing operand produces exit 1.
func TestDiffExitErrorMissingOperand(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		{
			Name:      "no_args",
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		{
			Name:      "group_only_no_file",
			Args:      []string{"staff"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffExitMixedSuccessFailure tests exit 1 when some files fail.
// R3.2: any error → exit 1, even if other files succeed.
func TestDiffExitMixedSuccessFailure(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	groupName := currentGroupName(t)
	errNorm := stderrNormalizer()

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "good.txt"), "ok", 0o644)
	writeFile(t, filepath.Join(goDir, "good.txt"), "ok", 0o644)

	// "bad.txt" does not exist — both binaries should error on it.
	refRes := runBin(t, refBin,
		[]string{groupName, "good.txt", "bad.txt"}, refDir)
	goRes := runBin(t, goBin,
		[]string{groupName, "good.txt", "bad.txt"}, goDir)

	refRes.stderr = errNorm(refRes.stderr)
	goRes.stderr = errNorm(goRes.stderr)
	compareResults(t, "mixed_success_failure", refRes, goRes)

	if goRes.exitCode != 1 {
		t.Errorf("expected exit 1 for mixed, got %d", goRes.exitCode)
	}
}

// TestDiffExitSuccessVerbose tests exit 0 with -v on a successful change.
// R3.2: success with verbose still exits 0.
func TestDiffExitSuccessVerbose(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	groupName := currentGroupName(t)

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "f.txt"), "x", 0o644)
	writeFile(t, filepath.Join(goDir, "f.txt"), "x", 0o644)

	refRes := runBin(t, refBin, []string{"-v", groupName, "f.txt"}, refDir)
	goRes := runBin(t, goBin, []string{"-v", groupName, "f.txt"}, goDir)

	refNorm := stdoutNormalizer(refDir)
	goNorm := stdoutNormalizer(goDir)
	refRes.stdout = refNorm(refRes.stdout)
	goRes.stdout = goNorm(goRes.stdout)
	compareResults(t, "exit_success_verbose", refRes, goRes)
}

// TestDiffExitErrorSilent tests exit 1 with -f on error.
// R3.2: silent mode still exits 1 on error.
func TestDiffExitErrorSilent(t *testing.T) {
	goBin, refBin := lookupBinaries(t)
	groupName := currentGroupName(t)
	errNorm := stderrNormalizer()

	refRes := runBin(t, refBin,
		[]string{"-f", groupName, "/no/such/file/chgrp_test"}, t.TempDir())
	goRes := runBin(t, goBin,
		[]string{"-f", groupName, "/no/such/file/chgrp_test"}, t.TempDir())

	refRes.stderr = errNorm(refRes.stderr)
	goRes.stderr = errNorm(goRes.stderr)
	compareResults(t, "exit_error_silent", refRes, goRes)
}

// --- R3.3: SIGPIPE handling ---
// R3.3 is verified structurally: sys.InstallSIGPIPEHandler() is called in main().
// The exit code tests above also exercise the process lifecycle which depends
// on correct signal handling. A dedicated SIGPIPE test would require piping
// verbose output to a process that closes early, but chgrp's output volume
// is too small to trigger SIGPIPE reliably in a differential test.

