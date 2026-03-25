// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd092-mkfifo R2.1-R2.3: Differential testing for mkfifo.
// R2.1: compare exit codes and stderr for basic FIFO creation.
// R2.2: compare behavior with -m/--mode flag and various octal values.
// R2.3: compare error cases (existing path, invalid mode).
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const binTimeout = 10 * time.Second

// makeBinNormalizer returns a normalizer that replaces binary paths and
// basenames with "mkfifo" so output from ref and go binaries can be compared.
func makeBinNormalizer(refBin, goBin string) testutils.NormalizeFunc {
	refBase := filepath.Base(refBin)
	goBase := filepath.Base(goBin)
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte("mkfifo"))
		b = bytes.ReplaceAll(b, []byte(goBin), []byte("mkfifo"))
		if refBase != "mkfifo" {
			b = bytes.ReplaceAll(b, []byte(refBase), []byte("mkfifo"))
		}
		if goBase != "mkfifo" {
			b = bytes.ReplaceAll(b, []byte(goBase), []byte("mkfifo"))
		}
		return b
	}
}

// setupWithFIFO creates a temp dir containing a pre-created FIFO.
func setupWithFIFO(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	createTestFIFO(t, filepath.Join(dir, name))
	return dir
}

// setupWithFile creates a temp dir containing a regular file.
func setupWithFile(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestDiff runs differential tests that work in a shared workdir.
// R2.1: verifies exit code 0 on success.
// R2.3: verifies error exit code and stderr parity.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkfifo")
	if err != nil {
		t.Skip("reference binary gmkfifo not in PATH")
	}

	norm := []testutils.NormalizeFunc{makeBinNormalizer(refBin, goBin)}

	tests := []testutils.DiffTest{
		{
			Name:      "missing_operand",
			Args:      []string{},
			Normalize: norm,
		},
		{
			Name:      "existing_fifo_error",
			Args:      []string{"pipe1"},
			WorkDir:   setupWithFIFO(t, "pipe1"),
			Normalize: norm,
		},
		{
			Name:      "existing_file_error",
			Args:      []string{"somefile"},
			WorkDir:   setupWithFile(t, "somefile"),
			Normalize: norm,
		},
		{
			Name:      "missing_parent_error",
			Args:      []string{"nodir/pipe1"},
			Normalize: norm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestMkfifoCreation runs each binary in separate temp dirs to test
// FIFO creation. R2.1: verifies basic creation. R2.2: verifies -m/--mode.
func TestMkfifoCreation(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkfifo")
	if err != nil {
		t.Skip("reference binary gmkfifo not in PATH")
	}

	normalize := makeBinNormalizer(refBin, goBin)

	tests := []struct {
		name string
		args []string
	}{
		{"single_fifo", []string{"pipe1"}},
		{"multiple_fifos", []string{"p1", "p2", "p3"}},
		{"mode_0600", []string{"-m", "0600", "pipe1"}},
		{"mode_0644", []string{"-m", "0644", "pipe1"}},
		{"mode_0755", []string{"-m", "0755", "pipe1"}},
		{"mode_long_flag", []string{"--mode=0600", "pipe1"}},
		{"mode_combined", []string{"-m0644", "pipe1"}},
		{"mode_multiple", []string{"-m", "0600", "p1", "p2"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareCreation(t, goBin, refBin, tc.args, normalize)
		})
	}
}

// binResult holds captured output from a binary invocation.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runBin executes a binary with args in workDir and captures output.
func runBin(t *testing.T, bin string, args []string, workDir string) binResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), binTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", bin)
	}
	return extractResult(t, bin, runErr, stdout.Bytes(), stderr.Bytes())
}

// extractResult converts exec results into a binResult.
func extractResult(
	t *testing.T, bin string, err error,
	stdout, stderr []byte,
) binResult {
	t.Helper()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", bin, err)
		}
	}
	return binResult{stdout: stdout, stderr: stderr, exitCode: code}
}

// compareCreation runs both binaries in separate dirs and compares output.
func compareCreation(
	t *testing.T, goBin, refBin string,
	args []string, normalize testutils.NormalizeFunc,
) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	refRes := runBin(t, refBin, args, refDir)
	goRes := runBin(t, goBin, args, goDir)

	refStdout := normalize(refRes.stdout)
	goStdout := normalize(goRes.stdout)
	refStderr := normalize(refRes.stderr)
	goStderr := normalize(goRes.stderr)

	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout differs\n  ref: %q\n  go:  %q", refStdout, goStdout)
	}
	if !bytes.Equal(refStderr, goStderr) {
		t.Errorf("stderr differs\n  ref: %q\n  go:  %q", refStderr, goStderr)
	}
	if refRes.exitCode != goRes.exitCode {
		t.Errorf("exit code differs: ref=%d go=%d",
			refRes.exitCode, goRes.exitCode)
	}
}

// TestMkfifoPermissions verifies permission bits match between Go and gmkfifo.
// R2.2: covers -m with various modes.
func TestMkfifoPermissions(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkfifo")
	if err != nil {
		t.Skip("reference binary gmkfifo not in PATH")
	}

	tests := []struct {
		name       string
		args       []string
		checkFifos []string
	}{
		{
			name:       "default_mode",
			args:       []string{"pipe1"},
			checkFifos: []string{"pipe1"},
		},
		{
			name:       "mode_0600",
			args:       []string{"-m", "0600", "pipe1"},
			checkFifos: []string{"pipe1"},
		},
		{
			name:       "mode_0644",
			args:       []string{"-m", "0644", "pipe1"},
			checkFifos: []string{"pipe1"},
		},
		{
			name:       "mode_0755",
			args:       []string{"-m", "0755", "pipe1"},
			checkFifos: []string{"pipe1"},
		},
		{
			name:       "mode_multiple_fifos",
			args:       []string{"-m", "0600", "p1", "p2"},
			checkFifos: []string{"p1", "p2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			comparePermissions(t, goBin, refBin, tc.args, tc.checkFifos)
		})
	}
}

// comparePermissions runs both binaries and compares FIFO permissions.
func comparePermissions(
	t *testing.T, goBin, refBin string,
	args, checkFifos []string,
) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	runBin(t, refBin, args, refDir)
	runBin(t, goBin, args, goDir)

	for _, f := range checkFifos {
		refPerm := fifoPerm(t, filepath.Join(refDir, f))
		goPerm := fifoPerm(t, filepath.Join(goDir, f))
		if refPerm != goPerm {
			t.Errorf("permissions differ for %s: ref=%o go=%o",
				f, refPerm, goPerm)
		}
	}
}

// fifoPerm returns the permission bits for a FIFO.
func fifoPerm(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}

// TestMkfifoPartialFailure verifies partial failure exits 1 but
// creates the valid FIFOs. R2.3: error cases with mixed valid/invalid.
func TestMkfifoPartialFailure(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkfifo")
	if err != nil {
		t.Skip("reference binary gmkfifo not in PATH")
	}

	normalize := makeBinNormalizer(refBin, goBin)

	// Create "existing" FIFO, then ask mkfifo to create both existing and new.
	refDir := t.TempDir()
	goDir := t.TempDir()
	createTestFIFO(t, filepath.Join(refDir, "existing"))
	createTestFIFO(t, filepath.Join(goDir, "existing"))

	args := []string{"existing", "newpipe"}
	refRes := runBin(t, refBin, args, refDir)
	goRes := runBin(t, goBin, args, goDir)

	refStderr := normalize(refRes.stderr)
	goStderr := normalize(goRes.stderr)

	if !bytes.Equal(refStderr, goStderr) {
		t.Errorf("stderr differs\n  ref: %q\n  go:  %q", refStderr, goStderr)
	}
	if refRes.exitCode != goRes.exitCode {
		t.Errorf("exit code differs: ref=%d go=%d",
			refRes.exitCode, goRes.exitCode)
	}

	// Verify newpipe was created in both dirs.
	verifyFIFOExists(t, filepath.Join(refDir, "newpipe"), "ref")
	verifyFIFOExists(t, filepath.Join(goDir, "newpipe"), "go")
}

// createTestFIFO creates a FIFO at the given path for test setup.
func createTestFIFO(t *testing.T, path string) {
	t.Helper()
	if err := syscall.Mkfifo(path, 0o666); err != nil {
		t.Fatalf("setup mkfifo %s: %v", path, err)
	}
}

// verifyFIFOExists checks that a FIFO was created at the given path.
func verifyFIFOExists(t *testing.T, path, label string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("%s: expected FIFO at %s: %v", label, path, err)
		return
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("%s: %s is not a FIFO (mode=%v)", label, path, info.Mode())
	}
}
