// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/chgrp against gchgrp (GNU coreutils).
// Implements srd090 R3.1-R3.3.
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gchgrp"
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

// currentGroupName returns the primary group name of the current user.
func currentGroupName(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}
	g, err := user.LookupGroupId(u.Gid)
	if err != nil {
		t.Fatalf("LookupGroupId %s: %v", u.Gid, err)
	}
	return g.Name
}

// secondaryGroupName returns a group name the current user belongs to
// that differs from the primary group. Returns "" if none found.
func secondaryGroupName(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}
	primaryGID, _ := strconv.Atoi(u.Gid)
	gids, err := os.Getgroups()
	if err != nil {
		return ""
	}
	for _, gid := range gids {
		if gid == primaryGID {
			continue
		}
		g, err := user.LookupGroupId(strconv.Itoa(gid))
		if err != nil {
			continue
		}
		return g.Name
	}
	return ""
}

// TestDiff runs differential tests comparing cmd/chgrp against gchgrp.
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
	t.Run("recursive", func(t *testing.T) {
		t.Parallel()
		runRecursiveTests(t, goBin, refBin, norm)
	})
	t.Run("multiple_files", func(t *testing.T) {
		t.Parallel()
		runMultipleFileTests(t, goBin, refBin, norm)
	})
	t.Run("error_continue", func(t *testing.T) {
		t.Parallel()
		runErrorContinueTests(t, goBin, refBin, norm)
	})
}

// runErrorTests tests error cases using RunDiffTests.
func runErrorTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	group := currentGroupName(t)
	norms := []testutils.NormalizeFunc{norm}
	tests := []testutils.DiffTest{
		{
			Name:      "nonexistent_file",
			Args:      []string{group, "nonexistent"},
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name:      "invalid_group",
			Args:      []string{"nonexistentgroup12345", "nonexistent"},
			ExitCode:  1,
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runVerboseTests tests R3.1: -v verbose diagnostic output.
func runVerboseTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	group := currentGroupName(t)
	t.Run("verbose_retained", func(t *testing.T) {
		t.Parallel()
		runChgrpInDirs(t, goBin, refBin, norm,
			[]string{"-v", group, "file"}, false)
	})
	altGroup := secondaryGroupName(t)
	if altGroup == "" {
		return
	}
	t.Run("verbose_changed", func(t *testing.T) {
		t.Parallel()
		refDir, goDir := setupPairedDirs(t, false)
		args := []string{"-v", altGroup, "file"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)
		compareOutputs(t, norm, refRes, goRes)
		compareFileGroup(t, refDir, goDir, "file")
	})
}

// runChangesTests tests R3.1: -c changes-only diagnostic output.
func runChangesTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	group := currentGroupName(t)
	t.Run("changes_no_change", func(t *testing.T) {
		t.Parallel()
		runChgrpInDirs(t, goBin, refBin, norm,
			[]string{"-c", group, "file"}, false)
	})
	altGroup := secondaryGroupName(t)
	if altGroup == "" {
		return
	}
	t.Run("changes_actual_change", func(t *testing.T) {
		t.Parallel()
		refDir, goDir := setupPairedDirs(t, false)
		args := []string{"-c", altGroup, "file"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)
		compareOutputs(t, norm, refRes, goRes)
		compareFileGroup(t, refDir, goDir, "file")
	})
}

// runSilentTests tests R3.1: -f silent/quiet error suppression.
func runSilentTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	group := currentGroupName(t)
	t.Run("silent_nonexistent", func(t *testing.T) {
		t.Parallel()
		refDir := t.TempDir()
		goDir := t.TempDir()
		args := []string{"-f", group, "nonexistent"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)
		compareOutputs(t, norm, refRes, goRes)
	})
}

// runRecursiveTests tests -R recursive group changes.
func runRecursiveTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	group := currentGroupName(t)
	t.Run("recursive_current_group", func(t *testing.T) {
		t.Parallel()
		runChgrpInDirs(t, goBin, refBin, norm,
			[]string{"-R", group, "testdir"}, true)
	})
	altGroup := secondaryGroupName(t)
	if altGroup == "" {
		return
	}
	t.Run("recursive_change_group", func(t *testing.T) {
		t.Parallel()
		refDir, goDir := setupPairedDirs(t, true)
		args := []string{"-R", altGroup, "testdir"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)
		compareOutputs(t, norm, refRes, goRes)
		compareFileGroup(t, refDir, goDir, filepath.Join("testdir", "file1"))
		compareFileGroup(t, refDir, goDir, filepath.Join("testdir", "sub", "file2"))
	})
}

// runMultipleFileTests tests R1.3: processing multiple FILE arguments.
func runMultipleFileTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	group := currentGroupName(t)
	t.Run("two_files", func(t *testing.T) {
		t.Parallel()
		refDir := t.TempDir()
		goDir := t.TempDir()
		setupFile(t, refDir, "file1")
		setupFile(t, refDir, "file2")
		setupFile(t, goDir, "file1")
		setupFile(t, goDir, "file2")
		args := []string{group, "file1", "file2"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)
		compareOutputs(t, norm, refRes, goRes)
	})
}

// runErrorContinueTests tests R1.4: continue processing after error.
func runErrorContinueTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	group := currentGroupName(t)
	t.Run("nonexistent_then_real", func(t *testing.T) {
		t.Parallel()
		refDir := t.TempDir()
		goDir := t.TempDir()
		setupFile(t, refDir, "realfile")
		setupFile(t, goDir, "realfile")
		args := []string{group, "nonexistent", "realfile"}
		refRes := runBin(t, refBin, args, refDir)
		goRes := runBin(t, goBin, args, goDir)
		compareOutputs(t, norm, refRes, goRes)
	})
}

// runChgrpInDirs runs a chgrp command in paired temp directories and compares.
func runChgrpInDirs(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc, args []string, useTree bool) {
	t.Helper()
	refDir, goDir := setupPairedDirs(t, useTree)
	refRes := runBin(t, refBin, args, refDir)
	goRes := runBin(t, goBin, args, goDir)
	compareOutputs(t, norm, refRes, goRes)
}

// setupPairedDirs creates matching temp directories for ref and go binaries.
// When useTree is true, creates a directory tree; otherwise a single file.
func setupPairedDirs(t *testing.T, useTree bool) (string, string) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	if useTree {
		setupTree(t, refDir)
		setupTree(t, goDir)
	} else {
		setupFile(t, refDir, "file")
		setupFile(t, goDir, "file")
	}
	return refDir, goDir
}

// setupFile creates a test file in the given directory.
func setupFile(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}
}

// setupTree creates a directory tree: root/testdir/file1, root/testdir/sub/file2.
func setupTree(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "testdir")
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("setup MkdirAll: %v", err)
	}
	setupFile(t, dir, "file1")
	setupFile(t, sub, "file2")
}

// compareFileGroup checks that a file has the same group in both dirs.
func compareFileGroup(t *testing.T, refDir, goDir, name string) {
	t.Helper()
	refGID := fileGID(t, filepath.Join(refDir, name))
	goGID := fileGID(t, filepath.Join(goDir, name))
	if refGID != goGID {
		t.Errorf("file %s: group mismatch ref=%d go=%d", name, refGID, goGID)
	}
}

// fileGID returns the group ID of a file.
func fileGID(t *testing.T, path string) uint32 {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat %s: %v", path, err)
	}
	return fi.Sys().(*syscall.Stat_t).Gid
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
