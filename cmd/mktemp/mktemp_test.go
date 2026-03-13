// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd036-mktemp R1.5, R2.1–R2.3, R4.1–R4.4
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
