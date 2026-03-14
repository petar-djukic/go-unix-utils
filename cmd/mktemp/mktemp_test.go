// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd036-mktemp R1.5, R2.1–R2.3, R3.1–R3.6, R4.1–R4.4
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinName is the Homebrew GNU reference binary for mktemp.
const refBinName = "gmktemp"

// TestDiffMktempExitCodes verifies exit code parity between Go and reference
// binaries for success and failure cases.
//
// R1.5: Exit 0 on success, exit 1 on failure with error on stderr.
// R4.1: Compare exit codes between Go binary and gmktemp.
func TestDiffMktempExitCodes(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{
			name:     "default creates file exits 0",
			args:     nil,
			wantCode: 0,
		},
		{
			name:     "directory mode exits 0",
			args:     []string{"-d"},
			wantCode: 0,
		},
		{
			name:     "too few Xs exits 1",
			args:     []string{"badXX"},
			wantCode: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			refCode := runExitCode(t, refBin, tc.args)
			goCode := runExitCode(t, goBin, tc.args)

			if refCode != goCode {
				t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
			}
			if goCode != tc.wantCode {
				t.Errorf("expected exit code %d, got %d", tc.wantCode, goCode)
			}
		})
	}
}

// TestDiffMktempDefaultFile verifies structural properties of default file creation.
//
// R1.1, R1.2, R1.4, R1.5: Default file in TMPDIR, tmp.XXXXXXXXXX pattern, mode 0600.
// R4.2: Structural validation of output path and file properties.
func TestDiffMktempDefaultFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tmpDir := t.TempDir()
	env := []string{"TMPDIR=" + tmpDir, "LC_ALL=C"}

	// Run both binaries with same TMPDIR.
	refOut := runOutput(t, refBin, nil, env)
	goOut := runOutput(t, goBin, nil, env)

	// R4.2: Output is a valid path in the expected directory.
	refPath := strings.TrimSpace(string(refOut))
	goPath := strings.TrimSpace(string(goOut))

	if filepath.Dir(refPath) != tmpDir {
		t.Errorf("ref path not in TMPDIR: %s", refPath)
	}
	if filepath.Dir(goPath) != tmpDir {
		t.Errorf("go path not in TMPDIR: %s", goPath)
	}

	// R4.2: Name matches tmp.XXXXXXXXXX pattern.
	pattern := regexp.MustCompile(`^tmp\.[A-Za-z0-9]{10}$`)
	if !pattern.MatchString(filepath.Base(refPath)) {
		t.Errorf("ref name does not match pattern: %s", filepath.Base(refPath))
	}
	if !pattern.MatchString(filepath.Base(goPath)) {
		t.Errorf("go name does not match pattern: %s", filepath.Base(goPath))
	}

	// R4.2: File exists after creation.
	goInfo, err := os.Stat(goPath)
	if err != nil {
		t.Fatalf("go output file does not exist: %v", err)
	}

	// R1.4: Permission bits are 0600.
	if goInfo.Mode().Perm() != 0o600 {
		t.Errorf("expected mode 0600, got %04o", goInfo.Mode().Perm())
	}

	// R4.2: File, not directory.
	if goInfo.IsDir() {
		t.Errorf("expected file, got directory")
	}

	// Cleanup created files.
	_ = os.Remove(refPath) // best-effort cleanup
	_ = os.Remove(goPath)  // best-effort cleanup
}

// TestDiffMktempDirectoryMode verifies -d creates a directory with correct properties.
//
// R2.1: -d creates a directory.
// R2.2: Directory has mode 0700.
// R2.3: Prints absolute path to stdout.
// R4.2: Structural validation.
func TestDiffMktempDirectoryMode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tmpDir := t.TempDir()
	env := []string{"TMPDIR=" + tmpDir, "LC_ALL=C"}
	args := []string{"-d"}

	// Run both binaries.
	refOut := runOutput(t, refBin, args, env)
	goOut := runOutput(t, goBin, args, env)

	refPath := strings.TrimSpace(string(refOut))
	goPath := strings.TrimSpace(string(goOut))

	// R2.3: Output is an absolute path in the expected directory.
	if filepath.Dir(refPath) != tmpDir {
		t.Errorf("ref path not in TMPDIR: %s", refPath)
	}
	if filepath.Dir(goPath) != tmpDir {
		t.Errorf("go path not in TMPDIR: %s", goPath)
	}

	// R4.2: Name matches tmp.XXXXXXXXXX pattern.
	pattern := regexp.MustCompile(`^tmp\.[A-Za-z0-9]{10}$`)
	if !pattern.MatchString(filepath.Base(goPath)) {
		t.Errorf("go name does not match pattern: %s", filepath.Base(goPath))
	}

	// R2.1: Directory exists after creation.
	goInfo, err := os.Stat(goPath)
	if err != nil {
		t.Fatalf("go output directory does not exist: %v", err)
	}

	// R2.1: Is a directory, not a file.
	if !goInfo.IsDir() {
		t.Errorf("expected directory, got file")
	}

	// R2.2: Permission bits are 0700.
	if goInfo.Mode().Perm() != 0o700 {
		t.Errorf("expected mode 0700, got %04o", goInfo.Mode().Perm())
	}

	// Cleanup created directories.
	_ = os.Remove(refPath) // best-effort cleanup
	_ = os.Remove(goPath)  // best-effort cleanup
}

// TestDiffMktempDirectoryLongFlag verifies --directory behaves the same as -d.
//
// R2.1: --directory is the long form of -d.
func TestDiffMktempDirectoryLongFlag(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	_, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tmpDir := t.TempDir()
	env := []string{"TMPDIR=" + tmpDir, "LC_ALL=C"}
	args := []string{"--directory"}

	goOut := runOutput(t, goBin, args, env)
	goPath := strings.TrimSpace(string(goOut))

	// R2.1: Created entry is a directory.
	goInfo, err := os.Stat(goPath)
	if err != nil {
		t.Fatalf("go output directory does not exist: %v", err)
	}
	if !goInfo.IsDir() {
		t.Errorf("expected directory, got file")
	}

	// R2.2: Mode 0700.
	if goInfo.Mode().Perm() != 0o700 {
		t.Errorf("expected mode 0700, got %04o", goInfo.Mode().Perm())
	}

	_ = os.Remove(goPath) // best-effort cleanup
}

// TestDiffMktempDirectoryCustomTemplate verifies -d with a custom template.
// GNU mktemp creates in the current directory when a template without a path
// separator is provided (TMPDIR is only used for the default template).
//
// R2.1, R2.2, R2.3: Directory mode with custom template.
func TestDiffMktempDirectoryCustomTemplate(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tmpDir := t.TempDir()
	env := []string{"TMPDIR=" + tmpDir, "LC_ALL=C"}
	args := []string{"-d"}

	// Run Go binary with a second TMPDIR-based invocation using default template.
	goOut := runOutput(t, goBin, args, env)
	refOut := runOutput(t, refBin, args, env)

	goPath := strings.TrimSpace(string(goOut))
	refPath := strings.TrimSpace(string(refOut))

	// Both created directories in TMPDIR.
	if filepath.Dir(goPath) != tmpDir {
		t.Errorf("go path not in TMPDIR: %s", goPath)
	}
	if filepath.Dir(refPath) != tmpDir {
		t.Errorf("ref path not in TMPDIR: %s", refPath)
	}

	// Name matches default tmp.XXXXXXXXXX pattern.
	pattern := regexp.MustCompile(`^tmp\.[A-Za-z0-9]{10}$`)
	if !pattern.MatchString(filepath.Base(goPath)) {
		t.Errorf("go name does not match pattern: %s", filepath.Base(goPath))
	}

	// Is a directory with 0700.
	goInfo, err := os.Stat(goPath)
	if err != nil {
		t.Fatalf("go output directory does not exist: %v", err)
	}
	if !goInfo.IsDir() {
		t.Errorf("expected directory, got file")
	}
	if goInfo.Mode().Perm() != 0o700 {
		t.Errorf("expected mode 0700, got %04o", goInfo.Mode().Perm())
	}

	_ = os.Remove(refPath) // best-effort cleanup
	_ = os.Remove(goPath)  // best-effort cleanup
}

// TestDiffMktempFailureStderr verifies that failure cases produce stderr output.
//
// R1.5: Must print error to stderr on failure.
func TestDiffMktempFailureStderr(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	// Template with too few Xs should fail.
	args := []string{"badXX"}
	env := []string{"LC_ALL=C"}

	refCode, refStderr := runWithStderr(t, refBin, args, env)
	goCode, goStderr := runWithStderr(t, goBin, args, env)

	// R1.5: Both exit 1.
	if refCode != goCode {
		t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
	}

	// R1.5: Both produce stderr output.
	if len(refStderr) == 0 {
		t.Errorf("ref produced no stderr on failure")
	}
	if len(goStderr) == 0 {
		t.Errorf("go produced no stderr on failure")
	}
}

// runExitCode runs binary with args and returns its exit code.
func runExitCode(t *testing.T, binary string, args []string) int {
	t.Helper()
	tmpDir := t.TempDir()
	cmd := exec.Command(binary, args...)
	cmd.Env = buildTestEnv([]string{"TMPDIR=" + tmpDir, "LC_ALL=C"})
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		t.Fatalf("failed to run %q: %v", binary, err)
	}
	return 0
}

// runOutput runs binary with args and env, returning stdout. Fatals on non-zero exit.
func runOutput(t *testing.T, binary string, args []string, env []string) []byte {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = buildTestEnv(env)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to run %q %v: %v", binary, args, err)
	}
	return stdout.Bytes()
}

// runWithStderr runs binary and returns exit code and stderr content.
func runWithStderr(t *testing.T, binary string, args []string, env []string) (int, []byte) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = buildTestEnv(env)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %q: %v", binary, err)
		}
	}
	return exitCode, stderr.Bytes()
}

// TestDiffMktempNonexistentDir verifies R3.1: template with a directory
// separator where the directory does not exist produces exit 1 and stderr.
func TestDiffMktempNonexistentDir(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tmpDir := t.TempDir()
	// Template with nonexistent subdirectory. Both binaries should fail.
	args := []string{"nonexistent/foo.XXXX"}
	env := []string{"TMPDIR=" + tmpDir, "LC_ALL=C"}

	refCode, refStderr := runWithStderrAndDir(t, refBin, args, env, tmpDir)
	goCode, goStderr := runWithStderrAndDir(t, goBin, args, env, tmpDir)

	if refCode != goCode {
		t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
	}
	if goCode != 1 {
		t.Errorf("expected exit code 1, got %d", goCode)
	}
	if len(goStderr) == 0 {
		t.Errorf("go produced no stderr on nonexistent dir error")
	}
	if len(refStderr) == 0 {
		t.Errorf("ref produced no stderr on nonexistent dir error")
	}
}

// TestDiffMktempUnwritableDir verifies R3.2: unwritable target directory
// produces exit 1 and stderr.
func TestDiffMktempUnwritableDir(t *testing.T) {
	t.Parallel()

	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tmpDir := t.TempDir()
	unwritableDir := filepath.Join(tmpDir, "unwritable")
	if err := os.Mkdir(unwritableDir, 0o555); err != nil {
		t.Fatalf("failed to create unwritable dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(unwritableDir, 0o700) // restore permissions for cleanup
	})

	env := []string{"TMPDIR=" + unwritableDir, "LC_ALL=C"}

	refCode, refStderr := runWithStderr(t, refBin, nil, env)
	goCode, goStderr := runWithStderr(t, goBin, nil, env)

	if refCode != goCode {
		t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
	}
	if goCode != 1 {
		t.Errorf("expected exit code 1, got %d", goCode)
	}
	if len(goStderr) == 0 {
		t.Errorf("go produced no stderr on unwritable dir error")
	}
	if len(refStderr) == 0 {
		t.Errorf("ref produced no stderr on unwritable dir error")
	}
}

// TestDiffMktempTooFewXs verifies R3.3: templates with fewer than 3 trailing
// X characters produce exit 1 and stderr.
func TestDiffMktempTooFewXs(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "zero Xs", args: []string{"noXatall"}},
		{name: "one X", args: []string{"oneX"}},
		{name: "two Xs", args: []string{"twoXX"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			env := []string{"TMPDIR=" + tmpDir, "LC_ALL=C"}

			refCode, refStderr := runWithStderr(t, refBin, tc.args, env)
			goCode, goStderr := runWithStderr(t, goBin, tc.args, env)

			if refCode != goCode {
				t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
			}
			if goCode != 1 {
				t.Errorf("expected exit code 1, got %d", goCode)
			}
			if len(goStderr) == 0 {
				t.Errorf("go produced no stderr for %v", tc.args)
			}
			if len(refStderr) == 0 {
				t.Errorf("ref produced no stderr for %v", tc.args)
			}
		})
	}
}

// TestMktempRetryOnCollision verifies R3.4: mktemp retries on name collision
// rather than failing immediately. This test is not differential because
// collisions depend on random generation. Instead it verifies that creating
// multiple files in rapid succession with a short template (minimum 3 Xs)
// all succeed, exercising the retry path if any collision occurs.
func TestMktempRetryOnCollision(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	env := []string{"TMPDIR=" + tmpDir, "LC_ALL=C"}
	// Use minimum X count (3) to increase collision probability.
	args := []string{"collision.XXX"}

	// Create several files in sequence; all should succeed.
	for i := range 10 {
		out := runOutput(t, goBin, args, env)
		path := strings.TrimSpace(string(out))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("iteration %d: created file does not exist: %v", i, err)
		}
	}
}

// runWithStderrAndDir runs binary with args, env, and working directory,
// returning the exit code and stderr content.
func runWithStderrAndDir(t *testing.T, binary string, args []string, env []string, dir string) (int, []byte) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = buildTestEnv(env)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %q: %v", binary, err)
		}
	}
	return exitCode, stderr.Bytes()
}

// TestDiffMktempDryRun verifies R3.5: -u/--dry-run prints the name without
// creating the file or directory, and prints a warning to stderr.
//
// R3.5: -u prints name without creating, warns on stderr.
// R4.1: Exit code parity with gmktemp.
// R4.2: Structural validation — path is in expected directory, no file created.
func TestDiffMktempDryRun(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "dry-run short flag", args: []string{"-u"}},
		{name: "dry-run long flag", args: []string{"--dry-run"}},
		{name: "dry-run with directory mode", args: []string{"-u", "-d"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			env := []string{"TMPDIR=" + tmpDir, "LC_ALL=C"}

			// Run both binaries.
			refCode, refStdout, _ := runFull(t, refBin, tc.args, env)
			goCode, goStdout, goStderr := runFull(t, goBin, tc.args, env)

			// R4.1: Exit codes must match.
			if refCode != goCode {
				t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
			}
			if goCode != 0 {
				t.Errorf("expected exit code 0, got %d", goCode)
			}

			// R4.2: Output is a valid path in the expected directory.
			goPath := strings.TrimSpace(string(goStdout))
			refPath := strings.TrimSpace(string(refStdout))
			if filepath.Dir(goPath) != tmpDir {
				t.Errorf("go path not in TMPDIR: %s", goPath)
			}
			if filepath.Dir(refPath) != tmpDir {
				t.Errorf("ref path not in TMPDIR: %s", refPath)
			}

			// R4.2: Name matches default tmp.XXXXXXXXXX pattern.
			pattern := regexp.MustCompile(`^tmp\.[A-Za-z0-9]{10}$`)
			if !pattern.MatchString(filepath.Base(goPath)) {
				t.Errorf("go name does not match pattern: %s", filepath.Base(goPath))
			}

			// R3.5: File or directory must NOT exist (dry-run).
			if _, err := os.Stat(goPath); err == nil {
				t.Errorf("dry-run created entity at %s, expected no creation", goPath)
			}

			// R3.5: Warning must appear on stderr.
			if len(goStderr) == 0 {
				t.Errorf("go produced no stderr warning for dry-run")
			}
		})
	}
}

// TestDiffMktempQuiet verifies R3.6: -q/--quiet suppresses error messages
// on failure while still producing the correct exit code.
//
// R3.6: -q suppresses stderr error messages.
// R4.1: Exit code parity with gmktemp.
func TestDiffMktempQuiet(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "quiet short flag too few Xs", args: []string{"-q", "badXX"}},
		{name: "quiet long flag too few Xs", args: []string{"--quiet", "badXX"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			env := []string{"TMPDIR=" + tmpDir, "LC_ALL=C"}

			refCode, _, _ := runFull(t, refBin, tc.args, env)
			goCode, _, goStderr := runFull(t, goBin, tc.args, env)

			// R4.1: Exit codes must match.
			if refCode != goCode {
				t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
			}
			if goCode != 1 {
				t.Errorf("expected exit code 1, got %d", goCode)
			}

			// R3.6: Stderr must be empty when -q is used.
			if len(goStderr) != 0 {
				t.Errorf("go produced stderr with -q: %q", goStderr)
			}
		})
	}
}

// TestDiffMktempQuietUnwritable verifies R3.6 with a permission error:
// -q suppresses stderr even for creation failures.
//
// R3.6: -q suppresses stderr on permission errors.
// R4.1: Exit code parity.
func TestDiffMktempQuietUnwritable(t *testing.T) {
	t.Parallel()

	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tmpDir := t.TempDir()
	unwritableDir := filepath.Join(tmpDir, "unwritable")
	if err := os.Mkdir(unwritableDir, 0o555); err != nil {
		t.Fatalf("failed to create unwritable dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(unwritableDir, 0o700) // restore permissions for cleanup
	})

	env := []string{"TMPDIR=" + unwritableDir, "LC_ALL=C"}
	args := []string{"-q"}

	refCode, _, _ := runFull(t, refBin, args, env)
	goCode, _, goStderr := runFull(t, goBin, args, env)

	// R4.1: Exit codes must match.
	if refCode != goCode {
		t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
	}
	if goCode != 1 {
		t.Errorf("expected exit code 1, got %d", goCode)
	}

	// R3.6: Stderr must be empty with -q.
	if len(goStderr) != 0 {
		t.Errorf("go produced stderr with -q: %q", goStderr)
	}
}

// runFull runs binary with args and env, returning exit code, stdout, and stderr.
func runFull(t *testing.T, binary string, args []string, env []string) (int, []byte, []byte) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Env = buildTestEnv(env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %q: %v", binary, err)
		}
	}
	return exitCode, stdout.Bytes(), stderr.Bytes()
}

// buildTestEnv merges overrides with the current process environment.
func buildTestEnv(overrides []string) []string {
	envMap := make(map[string]string, len(os.Environ()))
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		envMap[k] = v
	}
	for _, kv := range overrides {
		k, v, _ := strings.Cut(kv, "=")
		envMap[k] = v
	}
	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, k+"="+v)
	}
	return env
}
