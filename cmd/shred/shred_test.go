// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/shred against gshred (GNU coreutils).
//
// Covers prd099-shred R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const testTimeout = 10 * time.Second

// discardAll blanks all output so tests compare only exit code.
func discardAll(data []byte) []byte {
	return nil
}

// TestDiffErrors runs error-case tests via RunDiffTests (no file mutation).
// R1.1: missing file operand exits 1.
func TestDiffErrors(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshred")
	if err != nil {
		t.Skip("reference binary gshred not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			Name:      "no_arguments",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		{
			Name:      "nonexistent_file",
			Args:      []string{"no_such_file"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// shredTestCase describes a differential shred test with per-binary file setup.
type shredTestCase struct {
	name    string
	args    []string
	content string // initial file content
	removed bool   // expect file removed after shred
}

// TestDiffShred verifies file-mutating shred operations.
// Each binary runs in its own temp directory with its own copy of the test file.
func TestDiffShred(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshred")
	if err != nil {
		t.Skip("reference binary gshred not in PATH")
	}

	cases := []shredTestCase{
		// R1.1: default 3-pass random overwrite
		{
			name:    "R1.1_default_overwrite",
			args:    []string{"testfile"},
			content: "hello world\n",
		},
		// R1.2: custom iteration count
		{
			name:    "R1.2_iterations_1",
			args:    []string{"-n", "1", "testfile"},
			content: "data to overwrite\n",
		},
		// R1.2: long form --iterations=
		{
			name:    "R1.2_iterations_long",
			args:    []string{"--iterations=2", "testfile"},
			content: "long form test\n",
		},
		// R1.3: zero pass hides shredding
		{
			name:    "R1.3_zero_pass",
			args:    []string{"-n", "1", "-z", "testfile"},
			content: "test content\n",
		},
		// R1.4: remove after overwriting
		{
			name:    "R1.4_remove",
			args:    []string{"-u", "testfile"},
			content: "remove me\n",
			removed: true,
		},
		// R1.2+R1.3+R1.4: combined flags
		{
			name:    "R1.2_R1.3_R1.4_combined",
			args:    []string{"-n", "1", "-z", "-u", "testfile"},
			content: "combined test\n",
			removed: true,
		},
		// R1.1: empty file
		{
			name:    "R1.1_empty_file",
			args:    []string{"testfile"},
			content: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runShredCase(t, goBin, refBin, tc)
		})
	}
}

// runShredCase runs one shred test case against both binaries independently.
func runShredCase(t *testing.T, goBin, refBin string, tc shredTestCase) {
	t.Helper()

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeTestFile(t, filepath.Join(refDir, "testfile"), tc.content)
	writeTestFile(t, filepath.Join(goDir, "testfile"), tc.content)

	env := append(os.Environ(), "LC_ALL=C")

	_, refErr, refExit := runBin(t, refBin, tc.args, env, refDir)
	goOut, goErr, goExit := runBin(t, goBin, tc.args, env, goDir)

	compareResults(t, goOut, goErr, goExit, refErr, refExit)
	checkFileState(t, tc, goDir, refDir)
}

// compareResults checks exit code and stdout parity.
func compareResults(
	t *testing.T,
	goOut, goErr []byte, goExit int,
	refErr []byte, refExit int,
) {
	t.Helper()
	if len(goOut) != 0 {
		t.Errorf("go stdout should be empty, got %q", goOut)
	}
	if refExit != goExit {
		t.Errorf("exit code divergence: ref=%d go=%d\nref stderr: %q\ngo stderr: %q",
			refExit, goExit, refErr, goErr)
	}
}

// checkFileState verifies file existence or removal after shred.
func checkFileState(t *testing.T, tc shredTestCase, goDir, refDir string) {
	t.Helper()
	if tc.removed {
		assertAbsent(t, filepath.Join(goDir, "testfile"), "go")
		assertAbsent(t, filepath.Join(refDir, "testfile"), "ref")
	} else {
		assertExists(t, filepath.Join(goDir, "testfile"), "go")
		assertExists(t, filepath.Join(refDir, "testfile"), "ref")
	}
}

// TestZeroPassContent verifies that -z leaves the file filled with zeros.
// R1.3: final zero pass overwrites file content with null bytes.
func TestZeroPassContent(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	path := filepath.Join(dir, "zerofile")
	writeTestFile(t, path, "overwrite me please\n")

	env := append(os.Environ(), "LC_ALL=C")
	_, _, exit := runBin(t, goBin, []string{"-n", "0", "-z", path}, env, dir)
	if exit != 0 {
		t.Fatalf("shred -n 0 -z exited %d, want 0", exit)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading zeroed file: %v", err)
	}
	expected := make([]byte, len("overwrite me please\n"))
	if !bytes.Equal(data, expected) {
		t.Errorf("file not zeroed: got %q", data)
	}
}

// runBin executes a binary and returns stdout, stderr, exit code.
func runBin(t *testing.T, bin string, args []string, env []string, dir string) ([]byte, []byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out after %v", bin, testTimeout)
	}

	return stdout.Bytes(), stderr.Bytes(), exitCode(err)
}

// exitCode extracts the exit code from an error.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}

// assertAbsent verifies that a file does not exist.
func assertAbsent(t *testing.T, path, label string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("%s: file %s should not exist after -u", label, filepath.Base(path))
	}
}

// assertExists verifies that a file exists.
func assertExists(t *testing.T, path, label string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("%s: file %s should still exist: %v", label, filepath.Base(path), err)
	}
}
