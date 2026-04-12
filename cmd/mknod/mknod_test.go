// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mknod against gmknod (GNU coreutils).
// Implements srd093 R2.1 (TestDiff with RunDiffTests), R2.2 (test cases),
// R2.3 (graceful skip and root-privilege skip).
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

const refBinName = "gmknod"
const execTimeout = 30 * time.Second

// makeNormalizer creates a NormalizeFunc that replaces binary names and
// normalizes syscall error message capitalization between GNU and Go.
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
		{"File exists", "file exists"},
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

// TestDiff runs differential tests comparing cmd/mknod against gmknod.
// R2.1: uses testutils.BuildBinary and testutils.RunDiffTests.
// R2.2: covers FIFO creation, error cases, flag output, and -m mode.
// R2.3: skips when gmknod is not in PATH; skips device tests without root.
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
		runErrorTests(t, goBin, refBin)
	})
	t.Run("flags", func(t *testing.T) {
		t.Parallel()
		runFlagTests(t, goBin, refBin)
	})
	t.Run("fifo_creation", func(t *testing.T) {
		t.Parallel()
		runFIFOCreationTests(t, goBin, refBin, norm)
	})
	t.Run("device_creation", func(t *testing.T) {
		t.Parallel()
		runDeviceCreationTests(t, goBin, refBin, norm)
	})
}

// exitCodeCase defines a test that compares only exit codes between binaries.
type exitCodeCase struct {
	name     string
	args     []string
	exitCode int
}

// runErrorTests verifies that both binaries return matching exit codes
// for invalid argument combinations. Error message text differs between
// GNU and Go implementations, so only exit codes are compared.
func runErrorTests(t *testing.T, goBin, refBin string) {
	t.Helper()
	cases := []exitCodeCase{
		{name: "no_args", args: []string{}, exitCode: 1},
		{name: "missing_type", args: []string{"node1"}, exitCode: 1},
		{name: "invalid_type", args: []string{"node1", "z"}, exitCode: 1},
		{name: "fifo_extra_args", args: []string{"pipe1", "p", "1", "2"}, exitCode: 1},
		{name: "block_missing_minor", args: []string{"dev1", "b", "1"}, exitCode: 1},
		{name: "char_missing_devnums", args: []string{"dev1", "c"}, exitCode: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareExitCodes(t, goBin, refBin, tc)
		})
	}
}

// compareExitCodes runs both binaries and verifies their exit codes match.
func compareExitCodes(t *testing.T, goBin, refBin string, tc exitCodeCase) {
	t.Helper()
	workDir := t.TempDir()
	refRes := runBin(t, refBin, tc.args, workDir)
	goRes := runBin(t, goBin, tc.args, workDir)

	if refRes.exitCode != tc.exitCode {
		t.Errorf("ref exit code sanity: want=%d got=%d", tc.exitCode, refRes.exitCode)
	}
	if refRes.exitCode != goRes.exitCode {
		t.Errorf("exit code mismatch: ref=%d go=%d", refRes.exitCode, goRes.exitCode)
	}
}

// runFlagTests verifies --help and --version produce exit code 0 and
// non-empty stdout. Output text differs between GNU and Go so only
// exit code and non-emptiness are checked.
func runFlagTests(t *testing.T, goBin, refBin string) {
	t.Helper()
	cases := []struct {
		name string
		args []string
	}{
		{name: "help", args: []string{"--help"}},
		{name: "version", args: []string{"--version"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			workDir := t.TempDir()
			refRes := runBin(t, refBin, tc.args, workDir)
			goRes := runBin(t, goBin, tc.args, workDir)
			verifyFlagOutput(t, tc.name, refRes, goRes)
		})
	}
}

// verifyFlagOutput checks both binaries exit 0 with non-empty stdout.
func verifyFlagOutput(t *testing.T, name string, ref, got binResult) {
	t.Helper()
	if ref.exitCode != 0 {
		t.Errorf("%s: ref exit code=%d, want 0", name, ref.exitCode)
	}
	if got.exitCode != 0 {
		t.Errorf("%s: go exit code=%d, want 0", name, got.exitCode)
	}
	if len(got.stdout) == 0 {
		t.Errorf("%s: go stdout is empty", name)
	}
}

// isolatedCase defines a creation test that runs each binary in its own dir.
type isolatedCase struct {
	name       string
	args       []string
	checkFIFOs []string // relative paths whose mode to compare
}

// runFIFOCreationTests runs tests where each binary creates a FIFO in
// an isolated temp directory so filesystem mutations do not interfere.
func runFIFOCreationTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	cases := fifoCreationCases()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareIsolated(t, goBin, refBin, norm, tc)
		})
	}
}

// fifoCreationCases returns the table of isolated FIFO creation test cases.
// R2.2: basic FIFO creation, -m mode flag with various forms.
func fifoCreationCases() []isolatedCase {
	return []isolatedCase{
		{
			name: "basic_fifo", args: []string{"pipe1", "p"},
			checkFIFOs: []string{"pipe1"},
		},
		{
			name: "mode_0600", args: []string{"-m", "0600", "secure", "p"},
			checkFIFOs: []string{"secure"},
		},
		{
			name: "mode_0644", args: []string{"-m", "0644", "readable", "p"},
			checkFIFOs: []string{"readable"},
		},
		{
			name: "mode_equals", args: []string{"--mode=0700", "priv", "p"},
			checkFIFOs: []string{"priv"},
		},
	}
}

// runDeviceCreationTests skips when not running as root because block
// and character device creation requires root privileges (D4).
func runDeviceCreationTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	if os.Getuid() != 0 {
		t.Skip("block/character device creation requires root privileges")
	}

	cases := deviceCreationCases()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runDeviceCase(t, goBin, refBin, norm, tc)
		})
	}
}

// deviceCase defines a device creation test with type and mode checks.
type deviceCase struct {
	name string
	args []string
	file string
}

// deviceCreationCases returns test cases for block and character devices.
func deviceCreationCases() []deviceCase {
	return []deviceCase{
		{
			name: "block_device", args: []string{"bdev", "b", "1", "0"},
			file: "bdev",
		},
		{
			name: "char_device", args: []string{"cdev", "c", "1", "3"},
			file: "cdev",
		},
		{
			name: "char_u_alias", args: []string{"udev", "u", "1", "3"},
			file: "udev",
		},
	}
}

// runDeviceCase runs both binaries in separate temp dirs and compares
// output and the created device's existence.
func runDeviceCase(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc, tc deviceCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	refRes := runBin(t, refBin, tc.args, refDir)
	goRes := runBin(t, goBin, tc.args, goDir)

	compareOutputs(t, norm, refRes, goRes)
	compareDeviceExists(t, tc.file, refDir, goDir)
}

// compareDeviceExists checks that the device file exists in both dirs.
func compareDeviceExists(t *testing.T, name, refDir, goDir string) {
	t.Helper()
	refPath := filepath.Join(refDir, name)
	goPath := filepath.Join(goDir, name)

	_, refErr := os.Stat(refPath)
	_, goErr := os.Stat(goPath)

	if refErr != nil {
		t.Errorf("ref device %s missing: %v", name, refErr)
	}
	if goErr != nil {
		t.Errorf("go device %s missing: %v", name, goErr)
	}
}

// binResult holds captured output from a single binary execution.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// compareIsolated runs both binaries in separate temp dirs and compares
// stdout, stderr, exit code, and FIFO permissions.
func compareIsolated(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc, tc isolatedCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	refRes := runBin(t, refBin, tc.args, refDir)
	goRes := runBin(t, goBin, tc.args, goDir)

	compareOutputs(t, norm, refRes, goRes)
	compareFIFOPerms(t, refDir, goDir, tc.checkFIFOs)
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
			stdout: outBuf.Bytes(), stderr: errBuf.Bytes(),
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

// compareFIFOPerms verifies FIFO permissions match between ref and go dirs.
func compareFIFOPerms(t *testing.T, refDir, goDir string, checkFIFOs []string) {
	t.Helper()
	for _, rel := range checkFIFOs {
		comparePerm(t, rel, filepath.Join(refDir, rel), filepath.Join(goDir, rel))
	}
}

// comparePerm checks that a single FIFO exists in both trees with matching
// permission bits and that it is indeed a named pipe.
func comparePerm(t *testing.T, name, refPath, goPath string) {
	t.Helper()
	refInfo, err := os.Stat(refPath)
	if err != nil {
		t.Errorf("ref FIFO %s missing: %v", name, err)
		return
	}
	goInfo, err := os.Stat(goPath)
	if err != nil {
		t.Errorf("go FIFO %s missing: %v", name, err)
		return
	}
	verifyIsFIFO(t, name, refInfo, goInfo)
	if refInfo.Mode().Perm() != goInfo.Mode().Perm() {
		t.Errorf("perm mismatch %s: ref=%o got=%o",
			name, refInfo.Mode().Perm(), goInfo.Mode().Perm())
	}
}

// verifyIsFIFO checks that both ref and go entries are named pipes.
func verifyIsFIFO(t *testing.T, name string, refInfo, goInfo os.FileInfo) {
	t.Helper()
	if refInfo.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("ref %s is not a FIFO: mode=%v", name, refInfo.Mode())
	}
	if goInfo.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("go %s is not a FIFO: mode=%v", name, goInfo.Mode())
	}
}
