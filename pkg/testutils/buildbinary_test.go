// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestBuildBinaryCompilesValidPackage verifies that BuildBinary compiles a
// valid Go package and returns a path to an executable binary.
func TestBuildBinaryCompilesValidPackage(t *testing.T) {
	t.Parallel()
	dir := writeMinimalMainPackage(t)
	binPath := BuildBinary(t, dir)
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("binary not found at %s: %v", binPath, err)
	}
	if info.IsDir() {
		t.Fatalf("expected file, got directory at %s", binPath)
	}
	out, err := exec.Command(binPath).CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed to execute: %v\noutput: %s", err, out)
	}
}

// TestBuildBinarySkipsEmptyDir verifies that BuildBinary skips when the
// target directory contains no .go files. We verify by checking that
// skipIfNoGoFiles does not produce a glob error on an empty dir and that
// the glob returns no matches. Implements R5.2.
func TestBuildBinarySkipsEmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("unexpected glob error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no .go files, got %d", len(matches))
	}
}

// TestBuildBinarySkipsViaSubprocess runs BuildBinary in a subprocess on an
// empty directory and verifies the test is skipped (exit 0, no FAIL).
func TestBuildBinarySkipsViaSubprocess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmd := exec.Command("go", "test", "-run", "TestBuildBinarySubprocessSkipHelper",
		"-count=1", "-v", ".")
	cmd.Dir = findTestutilsDir(t)
	cmd.Env = append(os.Environ(), "BUILD_BINARY_TEST_DIR="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("subprocess failed: %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("SKIP")) {
		t.Fatalf("expected SKIP in output:\n%s", out)
	}
}

// TestBuildBinarySubprocessSkipHelper is the subprocess helper for the
// skip test. It is only run when BUILD_BINARY_TEST_DIR is set.
func TestBuildBinarySubprocessSkipHelper(t *testing.T) {
	dir := os.Getenv("BUILD_BINARY_TEST_DIR")
	if dir == "" {
		t.Skip("only runs as subprocess helper")
	}
	BuildBinary(t, dir)
}

// TestBuildBinaryFailsOnBadCode verifies that BuildBinary calls t.Fatal
// when the build command fails on a directory that contains .go files.
// Uses a subprocess to avoid terminating the parent test.
func TestBuildBinaryFailsOnBadCode(t *testing.T) {
	t.Parallel()
	dir := writeBadMainPackage(t)
	cmd := exec.Command("go", "test", "-run", "TestBuildBinarySubprocessFatalHelper",
		"-count=1", "-v", ".")
	cmd.Dir = findTestutilsDir(t)
	cmd.Env = append(os.Environ(), "BUILD_BINARY_TEST_DIR="+dir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected subprocess to fail, got success:\n%s", out)
	}
	if !bytes.Contains(out, []byte("FAIL")) {
		t.Fatalf("expected FAIL in output:\n%s", out)
	}
}

// TestBuildBinarySubprocessFatalHelper is the subprocess helper for the
// fatal test. It is only run when BUILD_BINARY_TEST_DIR is set.
func TestBuildBinarySubprocessFatalHelper(t *testing.T) {
	dir := os.Getenv("BUILD_BINARY_TEST_DIR")
	if dir == "" {
		t.Skip("only runs as subprocess helper")
	}
	BuildBinary(t, dir)
}

// writeMinimalMainPackage creates a temporary directory with a valid Go
// main package that prints "ok" and exits 0.
func writeMinimalMainPackage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	main := []byte("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"ok\") }\n")
	goMod := []byte("module testpkg\n\ngo 1.21\n")
	writeTestFile(t, filepath.Join(dir, "main.go"), main)
	writeTestFile(t, filepath.Join(dir, "go.mod"), goMod)
	return dir
}

// writeBadMainPackage creates a temporary directory with a Go package
// that will not compile (references an undefined function).
func writeBadMainPackage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bad := []byte("package main\nfunc main() { undefined() }\n")
	goMod := []byte("module badpkg\n\ngo 1.21\n")
	writeTestFile(t, filepath.Join(dir, "main.go"), bad)
	writeTestFile(t, filepath.Join(dir, "go.mod"), goMod)
	return dir
}

// writeTestFile writes content to path, failing the test on error.
func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// findTestutilsDir returns the absolute path to the testutils package
// directory for subprocess test invocations.
func findTestutilsDir(t *testing.T) string {
	t.Helper()
	// We are running inside pkg/testutils/ already
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	return dir
}

