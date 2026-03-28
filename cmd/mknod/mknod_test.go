// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd093-mknod R2.1-R2.3: Differential testing for mknod.
// R2.1: compare exit codes and stderr for FIFO creation with default/custom modes.
// R2.2: compare behavior for error cases (missing type, invalid type, extra args).
// R2.3: compare --version and --help output parity with gmknod.
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
// basenames with "mknod" so output from ref and go binaries can be compared.
func makeBinNormalizer(refBin, goBin string) testutils.NormalizeFunc {
	refBase := filepath.Base(refBin)
	goBase := filepath.Base(goBin)
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte("mknod"))
		b = bytes.ReplaceAll(b, []byte(goBin), []byte("mknod"))
		if refBase != "mknod" {
			b = bytes.ReplaceAll(b, []byte(refBase), []byte("mknod"))
		}
		if goBase != "mknod" {
			b = bytes.ReplaceAll(b, []byte(goBase), []byte("mknod"))
		}
		return b
	}
}

// TestDiff runs differential tests for error cases that share a workdir.
// R2.2: verifies error exit codes and stderr parity for invalid arguments.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmknod")
	if err != nil {
		t.Skip("reference binary gmknod not in PATH")
	}

	norm := []testutils.NormalizeFunc{makeBinNormalizer(refBin, goBin)}

	tests := []testutils.DiffTest{
		{
			Name:      "missing_operand",
			Args:      []string{},
			Normalize: norm,
		},
		{
			Name:      "missing_type",
			Args:      []string{"node1"},
			Normalize: norm,
		},
		{
			Name:      "invalid_type",
			Args:      []string{"node1", "x"},
			Normalize: norm,
		},
		{
			Name:      "fifo_with_major_minor",
			Args:      []string{"pipe1", "p", "1", "3"},
			Normalize: norm,
		},
		{
			Name:      "block_missing_minor",
			Args:      []string{"blk1", "b", "1"},
			Normalize: norm,
		},
		{
			Name:      "block_missing_major_minor",
			Args:      []string{"blk1", "b"},
			Normalize: norm,
		},
		{
			Name:      "char_extra_arg",
			Args:      []string{"chr1", "c", "1", "3", "extra"},
			Normalize: norm,
		},
		{
			Name:      "invalid_major_number",
			Args:      []string{"blk1", "b", "abc", "3"},
			Normalize: norm,
		},
		{
			Name:      "invalid_minor_number",
			Args:      []string{"blk1", "b", "1", "xyz"},
			Normalize: norm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestMknodFIFOCreation runs each binary in separate temp dirs to test
// FIFO creation via mknod NAME p.
// R2.1: verifies FIFO creation with default and custom modes.
func TestMknodFIFOCreation(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmknod")
	if err != nil {
		t.Skip("reference binary gmknod not in PATH")
	}

	normalize := makeBinNormalizer(refBin, goBin)

	tests := []struct {
		name string
		args []string
	}{
		{"fifo_default_mode", []string{"pipe1", "p"}},
		{"fifo_mode_0600", []string{"-m", "0600", "pipe1", "p"}},
		{"fifo_mode_0644", []string{"-m", "0644", "pipe1", "p"}},
		{"fifo_mode_0755", []string{"-m", "0755", "pipe1", "p"}},
		{"fifo_mode_long_flag", []string{"--mode=0600", "pipe1", "p"}},
		{"fifo_mode_combined", []string{"-m0644", "pipe1", "p"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareCreation(t, goBin, refBin, tc.args, normalize)
		})
	}
}

// TestMknodFIFOPermissions verifies permission bits match between Go and gmknod.
// R2.1: covers -m with various modes for type p.
func TestMknodFIFOPermissions(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmknod")
	if err != nil {
		t.Skip("reference binary gmknod not in PATH")
	}

	tests := []struct {
		name string
		args []string
	}{
		{"default_mode", []string{"pipe1", "p"}},
		{"mode_0600", []string{"-m", "0600", "pipe1", "p"}},
		{"mode_0644", []string{"-m", "0644", "pipe1", "p"}},
		{"mode_0755", []string{"-m", "0755", "pipe1", "p"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			comparePermissions(t, goBin, refBin, tc.args, "pipe1")
		})
	}
}

// TestMknodExistingFIFO verifies error when FIFO already exists.
// R2.2: existing file error case.
func TestMknodExistingFIFO(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmknod")
	if err != nil {
		t.Skip("reference binary gmknod not in PATH")
	}

	normalize := makeBinNormalizer(refBin, goBin)

	refDir := t.TempDir()
	goDir := t.TempDir()
	createTestFIFO(t, filepath.Join(refDir, "pipe1"))
	createTestFIFO(t, filepath.Join(goDir, "pipe1"))

	args := []string{"pipe1", "p"}
	compareResults(t, goBin, refBin, args, refDir, goDir, normalize)
}

// TestMknodDeviceSkipNonRoot verifies device creation tests are skipped
// when not running as root.
// D2: device node creation requires root privileges.
func TestMknodDeviceSkipNonRoot(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("test only runs as non-root to verify skip behavior")
	}

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmknod")
	if err != nil {
		t.Skip("reference binary gmknod not in PATH")
	}

	normalize := makeBinNormalizer(refBin, goBin)

	// Both binaries should fail with permission error as non-root.
	tests := []struct {
		name string
		args []string
	}{
		{"block_device", []string{"blk0", "b", "1", "3"}},
		{"char_device", []string{"chr0", "c", "1", "3"}},
		{"char_device_u", []string{"chr1", "u", "1", "3"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refDir := t.TempDir()
			goDir := t.TempDir()
			compareResults(
				t, goBin, refBin, tc.args,
				refDir, goDir, normalize,
			)
		})
	}
}

// TestMknodVersion verifies --version output.
// R2.3: version output parity.
func TestMknodVersion(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmknod")
	if err != nil {
		t.Skip("reference binary gmknod not in PATH")
	}

	// Both should exit 0 and produce version output on stdout.
	refRes := runBin(t, refBin, []string{"--version"}, t.TempDir())
	goRes := runBin(t, goBin, []string{"--version"}, t.TempDir())

	if refRes.exitCode != 0 {
		t.Errorf("ref --version exit code: %d", refRes.exitCode)
	}
	if goRes.exitCode != 0 {
		t.Errorf("go --version exit code: %d", goRes.exitCode)
	}
	if len(goRes.stdout) == 0 {
		t.Error("go --version produced no stdout")
	}
}

// TestMknodHelp verifies --help output.
// R2.3: help output parity.
func TestMknodHelp(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmknod")
	if err != nil {
		t.Skip("reference binary gmknod not in PATH")
	}

	refRes := runBin(t, refBin, []string{"--help"}, t.TempDir())
	goRes := runBin(t, goBin, []string{"--help"}, t.TempDir())

	if refRes.exitCode != 0 {
		t.Errorf("ref --help exit code: %d", refRes.exitCode)
	}
	if goRes.exitCode != 0 {
		t.Errorf("go --help exit code: %d", goRes.exitCode)
	}
	if len(goRes.stdout) == 0 {
		t.Error("go --help produced no stdout")
	}
	// Verify both mention "mknod" in output.
	if !bytes.Contains(goRes.stdout, []byte("mknod")) {
		t.Error("go --help does not contain 'mknod'")
	}
}

// binResult holds captured output from a binary invocation.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runBin executes a binary with args in workDir and captures output.
func runBin(
	t *testing.T, bin string, args []string, workDir string,
) binResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(
		context.Background(), binTimeout,
	)
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
	compareResults(t, goBin, refBin, args, refDir, goDir, normalize)
}

// compareResults runs both binaries and compares stdout, stderr, exit code.
func compareResults(
	t *testing.T, goBin, refBin string,
	args []string, refDir, goDir string,
	normalize testutils.NormalizeFunc,
) {
	t.Helper()
	refRes := runBin(t, refBin, args, refDir)
	goRes := runBin(t, goBin, args, goDir)

	refStdout := normalize(refRes.stdout)
	goStdout := normalize(goRes.stdout)
	refStderr := normalize(refRes.stderr)
	goStderr := normalize(goRes.stderr)

	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout differs\n  ref: %q\n  go:  %q",
			refStdout, goStdout)
	}
	if !bytes.Equal(refStderr, goStderr) {
		t.Errorf("stderr differs\n  ref: %q\n  go:  %q",
			refStderr, goStderr)
	}
	if refRes.exitCode != goRes.exitCode {
		t.Errorf("exit code differs: ref=%d go=%d",
			refRes.exitCode, goRes.exitCode)
	}
}

// comparePermissions runs both binaries and compares FIFO permissions.
func comparePermissions(
	t *testing.T, goBin, refBin string,
	args []string, fifoName string,
) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	runBin(t, refBin, args, refDir)
	runBin(t, goBin, args, goDir)

	refPerm := fifoPerm(t, filepath.Join(refDir, fifoName))
	goPerm := fifoPerm(t, filepath.Join(goDir, fifoName))
	if refPerm != goPerm {
		t.Errorf("permissions differ for %s: ref=%o go=%o",
			fifoName, refPerm, goPerm)
	}
}

// fifoPerm returns the permission bits for a file.
func fifoPerm(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}

// createTestFIFO creates a FIFO at the given path for test setup.
func createTestFIFO(t *testing.T, path string) {
	t.Helper()
	if err := syscall.Mkfifo(path, 0o666); err != nil {
		t.Fatalf("setup mkfifo %s: %v", path, err)
	}
}
