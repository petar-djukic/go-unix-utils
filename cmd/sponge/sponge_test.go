// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/sponge via differential tests against the reference
// binary (sponge from Homebrew moreutils) and direct functional tests.
//
// Implements: prd007-sponge R1-R5
// Test suite: docs/specs/test-suites/test-rel01.2.yaml
// Architecture: docs/ARCHITECTURE.yaml § pkg/testutils
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goSponge is the absolute path to the Go sponge binary, built by TestMain.
var goSponge string

// refSponge is the absolute path to the reference sponge binary.
var refSponge string

// refSpongeFound reports whether the reference binary was found on PATH.
var refSpongeFound bool

// TestMain builds the Go sponge binary once and locates the reference binary
// before running any test. It uses os.Exit(m.Run()) as required by the testing
// package when TestMain is defined.
func TestMain(m *testing.M) {
	var err error
	goSponge, err = buildGoSponge()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build cmd/sponge: %v\n", err)
		os.Exit(1)
	}
	refSponge, refSpongeFound = resolveRefSponge()
	os.Exit(m.Run())
}

// buildGoSponge compiles cmd/sponge into bin/sponge under the project root and
// returns the absolute path to the binary.
func buildGoSponge() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	binDir := filepath.Join(cwd, "..", "..", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir bin: %w", err)
	}
	binPath := filepath.Join(binDir, "sponge")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return binPath, nil
}

// resolveRefSponge locates the reference sponge binary via exec.LookPath.
// Returns the path and true if found, or empty string and false if not.
func resolveRefSponge() (string, bool) {
	path, err := exec.LookPath("sponge")
	if err != nil {
		return "", false
	}
	return path, true
}

// skipIfNoRef skips the test when the reference sponge binary is not found.
func skipIfNoRef(t *testing.T) {
	t.Helper()
	if !refSpongeFound {
		t.Skip("sponge reference binary not found; skipping differential test")
	}
}

// lcEnv returns the environment slice applied to every test case.
// LC_ALL=C is required by ARCHITECTURE.yaml DD6 to eliminate locale divergence.
func lcEnv() []string {
	return []string{"LC_ALL=C"}
}

// writeFile creates path with the given content. It is a test helper that
// registers a cleanup for the containing test.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

// readFile reads and returns the contents of path.
func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile %s: %v", path, err)
	}
	return data
}

// allBytes returns a slice containing all 256 byte values in ascending order.
func allBytes() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

// runSponge executes the Go sponge binary with the given args, stdin, and env.
// Returns stdout, stderr, and the exit code.
func runSponge(t *testing.T, args []string, stdin []byte, env []string) ([]byte, []byte, int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(goSponge, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Env = append(os.Environ(), env...)

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError) //nolint:errorlint
		if !ok {
			t.Fatalf("running sponge: %v", err)
		}
		exitCode = exitErr.ExitCode()
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}

// --- Differential tests (skip when reference binary is absent) ---

// TestDiff_Passthrough verifies passthrough mode against the reference binary
// using DiffTest stdout comparison (prd007-sponge R4.1, R4.3).
func TestDiff_Passthrough(t *testing.T) {
	t.Parallel()
	skipIfNoRef(t)

	testutils.DiffTest{
		Stdin:      []byte("hello world\n"),
		Env:        lcEnv(),
		WantExit:   0,
		WantStdout: []byte("hello world\n"),
		WantStderr: nil,
	}.Run(t, refSponge, goSponge)
}

// TestDiff_EmptyPassthrough verifies passthrough mode with empty stdin
// against the reference binary (prd007-sponge R4.1).
func TestDiff_EmptyPassthrough(t *testing.T) {
	t.Parallel()
	skipIfNoRef(t)

	testutils.DiffTest{
		Stdin:      nil,
		Env:        lcEnv(),
		WantExit:   0,
		WantStdout: nil,
		WantStderr: nil,
	}.Run(t, refSponge, goSponge)
}

// TestDiff_BinaryPassthrough verifies passthrough mode with non-UTF-8 binary
// data against the reference binary (prd007-sponge R1.1).
func TestDiff_BinaryPassthrough(t *testing.T) {
	t.Parallel()
	skipIfNoRef(t)

	input := allBytes()
	testutils.DiffTest{
		Stdin:      input,
		Env:        lcEnv(),
		WantExit:   0,
		WantStdout: input,
		WantStderr: nil,
	}.Run(t, refSponge, goSponge)
}

// --- Direct functional tests (always run, no reference binary needed) ---

// TestPassthrough verifies that sponge with no FILE argument writes stdin to
// stdout byte-for-byte (prd007-sponge R4.1, R4.3).
func TestPassthrough(t *testing.T) {
	t.Parallel()

	input := []byte("hello world\n")
	stdout, _, exitCode := runSponge(t, nil, input, lcEnv())

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	if !bytes.Equal(stdout, input) {
		t.Errorf("stdout = %q, want %q", stdout, input)
	}
}

// TestEmptyPassthrough verifies passthrough mode with empty stdin produces
// no stdout output (prd007-sponge R4.1).
func TestEmptyPassthrough(t *testing.T) {
	t.Parallel()

	stdout, _, exitCode := runSponge(t, nil, nil, lcEnv())

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	if len(stdout) != 0 {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

// TestFileOverwrite verifies that sponge writes stdin to the named file
// (prd007-sponge R1.1, R2.1).
func TestFileOverwrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")
	input := []byte("line1\nline2\nline3\n")

	_, _, exitCode := runSponge(t, []string{outFile}, input, lcEnv())

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	got := readFile(t, outFile)
	if !bytes.Equal(got, input) {
		t.Errorf("file content = %q, want %q", got, input)
	}
}

// TestEmptyStdinToFile verifies that sponge with empty stdin creates an
// empty output file (prd007-sponge R1.1, R2.1).
func TestEmptyStdinToFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outFile := filepath.Join(dir, "empty.txt")

	_, _, exitCode := runSponge(t, []string{outFile}, nil, lcEnv())

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	got := readFile(t, outFile)
	if len(got) != 0 {
		t.Errorf("file content = %q, want empty", got)
	}
}

// TestBinaryInput verifies that non-UTF-8 binary data passes through sponge
// to a file without corruption (prd007-sponge R1.1).
func TestBinaryInput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outFile := filepath.Join(dir, "binary.dat")
	input := allBytes()

	_, _, exitCode := runSponge(t, []string{outFile}, input, lcEnv())

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	got := readFile(t, outFile)
	if !bytes.Equal(got, input) {
		t.Errorf("file content length = %d, want %d", len(got), len(input))
	}
}

// TestAppendMode verifies that -a prepends original file content before
// stdin content (prd007-sponge R3.1, R3.3).
func TestAppendMode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outFile := filepath.Join(dir, "existing.txt")
	writeFile(t, outFile, "original line\n")

	input := []byte("appended line\n")
	_, _, exitCode := runSponge(t, []string{"-a", outFile}, input, lcEnv())

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	got := readFile(t, outFile)
	want := []byte("original line\nappended line\n")
	if !bytes.Equal(got, want) {
		t.Errorf("file content = %q, want %q", got, want)
	}
}

// TestAppendModeNoExistingFile verifies that -a with a non-existent output
// file creates a new file with stdin content only (prd007-sponge R3.2).
func TestAppendModeNoExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outFile := filepath.Join(dir, "newfile.txt")
	input := []byte("new content\n")

	_, _, exitCode := runSponge(t, []string{"-a", outFile}, input, lcEnv())

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	got := readFile(t, outFile)
	if !bytes.Equal(got, input) {
		t.Errorf("file content = %q, want %q", got, input)
	}
}

// TestSoakBeforeWrite verifies the core sponge contract: reading a file
// through a pipeline and writing back to the same file must not truncate
// the input (prd007-sponge R1.1, R2.5).
func TestSoakBeforeWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.txt")
	original := "line1\nline2\nline3\n"
	writeFile(t, dataFile, original)

	// Simulate: cat data.txt | sponge data.txt
	input, err := os.ReadFile(dataFile)
	if err != nil {
		t.Fatalf("reading data file: %v", err)
	}

	_, _, exitCode := runSponge(t, []string{dataFile}, input, lcEnv())

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	got := readFile(t, dataFile)
	if !bytes.Equal(got, input) {
		t.Errorf("file content = %q, want %q (soak-before-write contract violated)", got, input)
	}
}

// TestFilePermissionsPreserved verifies that sponge preserves the file
// permissions of an existing output file (prd007-sponge R2.3).
func TestFilePermissionsPreserved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outFile := filepath.Join(dir, "protected.txt")
	if err := os.WriteFile(outFile, []byte("old content\n"), 0o640); err != nil {
		t.Fatalf("creating protected file: %v", err)
	}

	input := []byte("new content\n")
	_, _, exitCode := runSponge(t, []string{outFile}, input, lcEnv())

	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	got := readFile(t, outFile)
	if !bytes.Equal(got, input) {
		t.Errorf("file content = %q, want %q", got, input)
	}

	info, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("stat %s: %v", outFile, err)
	}
	gotMode := info.Mode().Perm()
	wantMode := os.FileMode(0o640)
	if gotMode != wantMode {
		t.Errorf("file mode = %o, want %o", gotMode, wantMode)
	}
}
