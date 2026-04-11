// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/truncate against gtruncate (GNU coreutils).
// Implements srd083 R1.1-R1.4, R2.1-R2.2, R3.1-R3.2.
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

const refBinName = "gtruncate"
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
		{"File exists", "file exists"},
		{"Permission denied", "permission denied"},
		{"Operation not permitted", "operation not permitted"},
		{"Invalid argument", "invalid argument"},
		{"Invalid number", "invalid number"},
	}
	for _, r := range replacements {
		b = bytes.ReplaceAll(b, []byte(r.from), []byte(r.to))
	}
	return b
}

// TestDiff runs differential tests comparing cmd/truncate against gtruncate.
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
	t.Run("sizing", func(t *testing.T) {
		t.Parallel()
		runSizingTests(t, goBin, refBin, norm)
	})
}

// runErrorTests tests error cases using RunDiffTests where no filesystem
// mutation occurs (both binaries fail before modifying files).
func runErrorTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	norms := []testutils.NormalizeFunc{norm}
	tests := []testutils.DiffTest{
		{
			Name: "missing_operand", Args: []string{"-s", "100"},
			ExitCode: 1, Normalize: norms,
		},
		{
			Name: "missing_size_and_ref", Args: []string{"file"},
			ExitCode: 1, Normalize: norms,
		},
		{
			Name: "invalid_size", Args: []string{"-s", "abc", "file"},
			ExitCode: 1, Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// isolatedCase defines a test that runs each binary in its own temp dir.
type isolatedCase struct {
	name  string
	args  []string
	setup func(t *testing.T, dir string)
	files []string // files whose sizes to compare after execution
}

// runSizingTests runs tests where each binary operates in an isolated
// temp directory since truncate modifies file sizes.
func runSizingTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	cases := sizingCases()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareIsolated(t, goBin, refBin, norm, tc)
		})
	}
}

// writeFile creates a file of the given size filled with zero bytes.
func writeFile(t *testing.T, dir, name string, size int64) {
	t.Helper()
	path := filepath.Join(dir, name)
	data := make([]byte, size)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("setup writeFile: %v", err)
	}
}

// sizingCases returns the table of isolated truncate test cases.
func sizingCases() []isolatedCase {
	return []isolatedCase{
		{
			name: "absolute_100",
			args: []string{"-s", "100", "testfile"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "testfile", 0)
			},
			files: []string{"testfile"},
		},
		{
			name: "create_missing",
			args: []string{"-s", "50", "newfile"},
			setup: func(t *testing.T, _ string) {
				t.Helper()
			},
			files: []string{"newfile"},
		},
		{
			name: "relative_grow",
			args: []string{"-s", "+50", "testfile"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "testfile", 100)
			},
			files: []string{"testfile"},
		},
		{
			name: "relative_shrink",
			args: []string{"-s", "-30", "testfile"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "testfile", 100)
			},
			files: []string{"testfile"},
		},
		{
			name: "suffix_K",
			args: []string{"-s", "1K", "testfile"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "testfile", 0)
			},
			files: []string{"testfile"},
		},
		{
			name: "reference_file",
			args: []string{"-r", "ref", "testfile"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "ref", 200)
				writeFile(t, dir, "testfile", 50)
			},
			files: []string{"testfile"},
		},
		{
			name: "reference_with_grow",
			args: []string{"-r", "ref", "-s", "+10", "testfile"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "ref", 200)
				writeFile(t, dir, "testfile", 50)
			},
			files: []string{"testfile"},
		},
		{
			name: "multiple_files",
			args: []string{"-s", "100", "file1", "file2"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "file1", 0)
				writeFile(t, dir, "file2", 50)
			},
			files: []string{"file1", "file2"},
		},
		{
			name: "no_create_missing",
			args: []string{"-c", "-s", "100", "nonexistent"},
			setup: func(t *testing.T, _ string) {
				t.Helper()
			},
			files: nil,
		},
		{
			name: "at_most_smaller",
			args: []string{"-s", "<50", "testfile"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "testfile", 100)
			},
			files: []string{"testfile"},
		},
		{
			name: "at_most_larger",
			args: []string{"-s", "<200", "testfile"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "testfile", 100)
			},
			files: []string{"testfile"},
		},
		{
			name: "at_least_larger",
			args: []string{"-s", ">200", "testfile"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "testfile", 100)
			},
			files: []string{"testfile"},
		},
	}
}

// compareIsolated runs both binaries in separate temp dirs and compares
// stdout, stderr, exit code, and resulting file sizes.
func compareIsolated(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc, tc isolatedCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	tc.setup(t, refDir)
	tc.setup(t, goDir)

	refRes := runBin(t, refBin, tc.args, refDir)
	goRes := runBin(t, goBin, tc.args, goDir)

	compareOutputs(t, norm, refRes, goRes)
	compareFileSizes(t, tc.files, refDir, goDir)
}

// compareFileSizes checks that files have the same size in both dirs.
func compareFileSizes(t *testing.T, files []string, refDir, goDir string) {
	t.Helper()
	for _, name := range files {
		refSize := fileSize(t, refDir, name, "ref")
		goSize := fileSize(t, goDir, name, "go")
		if refSize != goSize {
			t.Errorf("file size mismatch for %s: ref=%d go=%d",
				name, refSize, goSize)
		}
	}
}

// fileSize returns the size of a file in dir, or -1 if it does not exist.
func fileSize(t *testing.T, dir, name, label string) int64 {
	t.Helper()
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		t.Errorf("%s file missing: %s (%v)", label, name, err)
		return -1
	}
	return info.Size()
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
