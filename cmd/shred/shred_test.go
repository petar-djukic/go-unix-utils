// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/shred against gshred (GNU coreutils).
//
// Covers prd099-shred R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const testTimeout = 10 * time.Second

// discardAll blanks all output so tests compare only exit code.
func discardAll(data []byte) []byte {
	return nil
}

// normalizeProgName replaces gshred: with shred: for stderr comparison.
func normalizeProgName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gshred:"), []byte("shred:"))
}

// TestDiffErrors runs error-case tests via RunDiffTests (no file mutation).
// R1.1: missing file operand exits 1.
// R2.4: nonexistent file exits 1.
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
	name          string
	args          []string
	content       string // initial file content
	removed       bool   // expect file removed after shred
	compareStderr bool   // compare stderr output between binaries
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
		// R2.1: verbose output with single random pass
		{
			name:          "R2.1_verbose_1pass",
			args:          []string{"-v", "-n", "1", "testfile"},
			content:       "verbose test\n",
			compareStderr: true,
		},
		// R2.1: verbose with zero pass
		{
			name:          "R2.1_verbose_zero",
			args:          []string{"-v", "-n", "1", "-z", "testfile"},
			content:       "verbose zero\n",
			compareStderr: true,
		},
		// R2.1: verbose with default 3 passes
		{
			name:          "R2.1_verbose_default",
			args:          []string{"-v", "testfile"},
			content:       "three passes\n",
			compareStderr: true,
		},
		// R2.2: size option limits overwrite
		{
			name:    "R2.2_size_5",
			args:    []string{"--size=5", "testfile"},
			content: "ABCDEFGHIJ",
		},
		// R2.2: size with K suffix
		{
			name:    "R2.2_size_suffix_K",
			args:    []string{"-s", "1K", "testfile"},
			content: strings.Repeat("x", 2048),
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

	compareResults(t, goOut, goErr, goExit, refErr, refExit, tc.compareStderr)
	checkFileState(t, tc, goDir, refDir)
}

// compareResults checks exit code, stdout, and optionally stderr parity.
func compareResults(
	t *testing.T,
	goOut, goErr []byte, goExit int,
	refErr []byte, refExit int,
	checkStderr bool,
) {
	t.Helper()
	if len(goOut) != 0 {
		t.Errorf("go stdout should be empty, got %q", goOut)
	}
	if refExit != goExit {
		t.Errorf("exit code divergence: ref=%d go=%d\nref stderr: %q\ngo stderr: %q",
			refExit, goExit, refErr, goErr)
	}
	if checkStderr {
		normRef := normalizeProgName(refErr)
		if !bytes.Equal(goErr, normRef) {
			t.Errorf("stderr mismatch:\nref: %q\ngo:  %q", normRef, goErr)
		}
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

// TestSizeLimit verifies that --size limits overwrite to N bytes.
// R2.2: only the first N bytes are overwritten.
func TestSizeLimit(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	path := filepath.Join(dir, "sizefile")
	writeTestFile(t, path, "ABCDEFGHIJ")

	env := append(os.Environ(), "LC_ALL=C")
	_, _, exit := runBin(t, goBin, []string{"-n", "0", "-z", "--size=5", path}, env, dir)
	if exit != 0 {
		t.Fatalf("shred --size=5 exited %d, want 0", exit)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if len(data) != 10 {
		t.Fatalf("file size changed: got %d, want 10", len(data))
	}
	for i := 0; i < 5; i++ {
		if data[i] != 0 {
			t.Errorf("byte %d: got %d, want 0", i, data[i])
		}
	}
	if string(data[5:]) != "FGHIJ" {
		t.Errorf("remaining bytes changed: got %q, want %q", data[5:], "FGHIJ")
	}
}

// TestDiffMultipleFiles verifies shred processes multiple files.
// R2.3: each file is shredded in order.
func TestDiffMultipleFiles(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshred")
	if err != nil {
		t.Skip("reference binary gshred not in PATH")
	}

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeTestFile(t, filepath.Join(refDir, "file1"), "content1\n")
	writeTestFile(t, filepath.Join(refDir, "file2"), "content2\n")
	writeTestFile(t, filepath.Join(goDir, "file1"), "content1\n")
	writeTestFile(t, filepath.Join(goDir, "file2"), "content2\n")

	env := append(os.Environ(), "LC_ALL=C")
	args := []string{"file1", "file2"}

	_, _, refExit := runBin(t, refBin, args, env, refDir)
	goOut, _, goExit := runBin(t, goBin, args, env, goDir)

	if len(goOut) != 0 {
		t.Errorf("go stdout should be empty, got %q", goOut)
	}
	if refExit != goExit {
		t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
	}
	assertExists(t, filepath.Join(goDir, "file1"), "go")
	assertExists(t, filepath.Join(goDir, "file2"), "go")
}

// TestDiffErrorContinue verifies shred continues after an error.
// R2.4: exits 1 when any file fails, but processes remaining files.
func TestDiffErrorContinue(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshred")
	if err != nil {
		t.Skip("reference binary gshred not in PATH")
	}

	refDir := t.TempDir()
	goDir := t.TempDir()
	writeTestFile(t, filepath.Join(refDir, "file2"), "survive\n")
	writeTestFile(t, filepath.Join(goDir, "file2"), "survive\n")

	env := append(os.Environ(), "LC_ALL=C")
	args := []string{"nonexistent", "file2"}

	_, _, refExit := runBin(t, refBin, args, env, refDir)
	_, _, goExit := runBin(t, goBin, args, env, goDir)

	if refExit != goExit {
		t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
	}
	// file2 should still exist — it was processed despite file1 error
	assertExists(t, filepath.Join(goDir, "file2"), "go")
	assertExists(t, filepath.Join(refDir, "file2"), "ref")
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
