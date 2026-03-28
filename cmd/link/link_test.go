// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/link against GNU glink.
// Covers prd084-link R2.1 (--version), R2.2 (--help),
// R2.3 (differential tests for all link behaviors).
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeVersionString replaces version output with a sentinel so
// GNU and go-unix-utils version strings compare equal.
func normalizeVersionString(b []byte) []byte {
	if len(b) > 0 {
		return []byte("version output present\n")
	}
	return b
}

// normalizeHelpOutput replaces help output with a sentinel so
// GNU and go-unix-utils help text compare equal.
func normalizeHelpOutput(b []byte) []byte {
	if len(b) > 0 {
		return []byte("help output present\n")
	}
	return b
}

// stderrNormalizer normalizes error messages between GNU glink and Go link.
// Handles binary name differences, file paths, and capitalization.
func stderrNormalizer() testutils.NormalizeFunc {
	binName := regexp.MustCompile(`/[^\s:]+/g?link|glink`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	absPath := regexp.MustCompile(`'/[^']*'`)
	return func(b []byte) []byte {
		b = binName.ReplaceAll(b, []byte("link"))
		b = tryHelp.ReplaceAll(b, nil)
		b = absPath.ReplaceAll(b, []byte("'PATH'"))
		b = bytes.ToLower(b)
		return b
	}
}

// TestDiffErrors runs differential tests for error cases and flags
// where both binaries produce comparable output without filesystem mutation.
func TestDiffErrors(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("glink")
	if err != nil {
		t.Skipf("reference binary glink not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()
	versionNorm := []testutils.NormalizeFunc{normalizeVersionString}
	helpNorm := []testutils.NormalizeFunc{normalizeHelpOutput}

	tests := []testutils.DiffTest{
		// R2.1: --version prints version info and exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: versionNorm,
		},
		// R2.2: --help prints usage info and exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: helpNorm,
		},
		// R1.3: zero arguments.
		{
			Name:      "no_args",
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R1.3: one argument (missing second operand).
		{
			Name:      "one_arg",
			Args:      []string{"file1"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R1.3: extra arguments.
		{
			Name:      "extra_args",
			Args:      []string{"a", "b", "c"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R1.4: nonexistent source file.
		{
			Name:      "nonexistent_source",
			Args:      []string{"no_such_file_xyz", "dest_xyz"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// cmdResult holds captured output from a binary invocation.
type cmdResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runBin runs a binary with the given args and working directory.
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
	return cmdResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exitCode}
}

// compareResults asserts that ref and go binary outputs match.
func compareResults(t *testing.T, ref, got cmdResult) {
	t.Helper()
	if !bytes.Equal(ref.stdout, got.stdout) {
		t.Errorf("stdout mismatch\n  ref: %q\n  go:  %q", ref.stdout, got.stdout)
	}
	if !bytes.Equal(ref.stderr, got.stderr) {
		t.Errorf("stderr mismatch\n  ref: %q\n  go:  %q", ref.stderr, got.stderr)
	}
	if ref.exitCode != got.exitCode {
		t.Errorf("exit code mismatch: ref=%d go=%d", ref.exitCode, got.exitCode)
	}
}

// createTempFile creates a regular file in dir and returns its path.
func createTempFile(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("test content\n"), 0o644); err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	return p
}

// TestLinkSuccess verifies successful hard link creation.
// R1.1: link FILE1 FILE2 creates a hard link.
// R1.2: raw link(2) semantics apply.
func TestLinkSuccess(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("glink")
	if err != nil {
		t.Skipf("reference binary glink not in PATH: %v", err)
	}

	// Run reference binary in its own temp dir.
	refDir := t.TempDir()
	refSrc := createTempFile(t, refDir, "source")
	refDst := filepath.Join(refDir, "dest")
	refRes := runBin(t, refBin, []string{refSrc, refDst}, refDir)
	assertLinked(t, refSrc, refDst, "ref")

	// Run Go binary in its own temp dir.
	goDir := t.TempDir()
	goSrc := createTempFile(t, goDir, "source")
	goDst := filepath.Join(goDir, "dest")
	goRes := runBin(t, goBin, []string{goSrc, goDst}, goDir)
	assertLinked(t, goSrc, goDst, "go")

	// Compare outputs.
	compareResults(t, refRes, goRes)
}

// assertLinked verifies that dst exists and shares an inode with src.
func assertLinked(t *testing.T, src, dst, label string) {
	t.Helper()
	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Errorf("%s: stat source %s: %v", label, src, err)
		return
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		t.Errorf("%s: stat dest %s: %v", label, dst, err)
		return
	}
	if !os.SameFile(srcInfo, dstInfo) {
		t.Errorf("%s: %s and %s are not hard links", label, src, dst)
	}
}

// TestLinkExistingDest verifies error when destination already exists.
// R1.4: link(2) fails when FILE2 already exists.
func TestLinkExistingDest(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("glink")
	if err != nil {
		t.Skipf("reference binary glink not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	// Run reference binary with existing dest.
	refDir := t.TempDir()
	refSrc := createTempFile(t, refDir, "source")
	createTempFile(t, refDir, "dest")
	refRes := runBin(t, refBin, []string{refSrc, filepath.Join(refDir, "dest")}, refDir)

	// Run Go binary with existing dest.
	goDir := t.TempDir()
	goSrc := createTempFile(t, goDir, "source")
	createTempFile(t, goDir, "dest")
	goRes := runBin(t, goBin, []string{goSrc, filepath.Join(goDir, "dest")}, goDir)

	// Normalize and compare.
	refRes.stderr = errNorm(refRes.stderr)
	goRes.stderr = errNorm(goRes.stderr)
	compareResults(t, refRes, goRes)
}
