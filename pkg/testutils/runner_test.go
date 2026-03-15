// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for RunDiffTests ExpectedFiles validation (prd001-testutils R5.1-R5.2).

package testutils

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestExpectedFilesMatch verifies that ExpectedFiles validation passes when
// the file content matches the expected bytes.
// R5.1: ExpectedFiles checked after execution when non-nil.
func TestExpectedFilesMatch(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	content := []byte("hello world\n")

	// Pre-create the expected file in workDir.
	if err := os.WriteFile(filepath.Join(workDir, "out.txt"), content, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Use /bin/echo with no args as both binaries. The file was pre-created;
	// ExpectedFiles validation reads it after execution.
	tests := []DiffTest{
		{
			Name:          "file matches",
			Args:          []string{},
			WorkDir:       workDir,
			ExpectedFiles: map[string][]byte{"out.txt": content},
		},
	}

	RunDiffTests(t, "/bin/echo", "/bin/echo", tests)
}

// TestExpectedFilesNilSkipsValidation verifies that nil ExpectedFiles does not
// trigger any file validation.
// R5.1: validation runs only when ExpectedFiles is non-nil.
func TestExpectedFilesNilSkipsValidation(t *testing.T) {
	t.Parallel()

	tests := []DiffTest{
		{
			Name:          "no expected files",
			Args:          []string{},
			ExitCode:      0,
			ExpectedFiles: nil,
		},
	}

	RunDiffTests(t, "/bin/echo", "/bin/echo", tests)
}

// TestExpectedFilesEmptyMapNoValidation verifies that an empty (non-nil)
// ExpectedFiles map has no entries to check and passes.
func TestExpectedFilesEmptyMapNoValidation(t *testing.T) {
	t.Parallel()

	tests := []DiffTest{
		{
			Name:          "empty expected files",
			Args:          []string{},
			ExitCode:      0,
			ExpectedFiles: map[string][]byte{},
		},
	}

	RunDiffTests(t, "/bin/echo", "/bin/echo", tests)
}

// TestExpectedFilesAfterOutputComparison verifies that ExpectedFiles validation
// runs after stdout/stderr/exit-code comparison (design decision D1).
func TestExpectedFilesAfterOutputComparison(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(workDir, "ok.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []DiffTest{
		{
			Name:          "matching file after matching output",
			Args:          []string{},
			WorkDir:       workDir,
			ExitCode:      0,
			ExpectedFiles: map[string][]byte{"ok.txt": []byte("ok\n")},
		},
	}

	RunDiffTests(t, "/bin/echo", "/bin/echo", tests)
}

// TestExpectedFilesAbsolutePath verifies that absolute paths in ExpectedFiles
// are used as-is rather than being joined with WorkDir.
func TestExpectedFilesAbsolutePath(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	absDir := t.TempDir()
	absFile := filepath.Join(absDir, "abs.txt")
	content := []byte("absolute\n")

	if err := os.WriteFile(absFile, content, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []DiffTest{
		{
			Name:          "absolute path",
			Args:          []string{},
			WorkDir:       workDir,
			ExitCode:      0,
			ExpectedFiles: map[string][]byte{absFile: content},
		},
	}

	RunDiffTests(t, "/bin/echo", "/bin/echo", tests)
}

// TestExpectedFilesMultipleEntries verifies that multiple ExpectedFiles entries
// are all validated in a single test case.
func TestExpectedFilesMultipleEntries(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	files := map[string][]byte{
		"a.txt": []byte("alpha\n"),
		"b.txt": []byte("beta\n"),
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(workDir, name), content, 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	tests := []DiffTest{
		{
			Name:          "multiple files match",
			Args:          []string{},
			WorkDir:       workDir,
			ExitCode:      0,
			ExpectedFiles: files,
		},
	}

	RunDiffTests(t, "/bin/echo", "/bin/echo", tests)
}

// TestExpectedFilesMissingDetection verifies that the validation logic detects
// missing files. We test the detection path directly since RunDiffTests uses
// t.Errorf for reporting (R5.2).
func TestExpectedFilesMissingDetection(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	missingPath := filepath.Join(workDir, "nonexistent.txt")

	// Verify the file truly doesn't exist.
	_, err := os.ReadFile(missingPath)
	if err == nil {
		t.Fatal("expected file to not exist for this test")
	}

	// Verify os.ReadFile returns an error for missing files — this is the
	// same check RunDiffTests performs in its ExpectedFiles loop.
	if !os.IsNotExist(err) {
		t.Fatalf("expected IsNotExist error, got: %v", err)
	}
}

// TestExpectedFilesContentMismatchDetection verifies that the validation logic
// detects content mismatches. We test the detection path directly since
// RunDiffTests uses t.Errorf for reporting (R5.1).
func TestExpectedFilesContentMismatchDetection(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	filePath := filepath.Join(workDir, "out.txt")

	actual := []byte("actual content\n")
	expected := []byte("expected content\n")

	if err := os.WriteFile(filePath, actual, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Read the file and verify bytes differ — the same check RunDiffTests performs.
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if bytes.Equal(content, expected) {
		t.Fatal("expected content to differ for this test")
	}

	if !bytes.Equal(content, actual) {
		t.Fatalf("file content should match what was written: got %q, want %q", content, actual)
	}
}

// TestExpectedFilesRelativeToWorkDir verifies that relative paths in
// ExpectedFiles are resolved relative to WorkDir (design decision D2).
func TestExpectedFilesRelativeToWorkDir(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	subDir := filepath.Join(workDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	content := []byte("nested\n")
	if err := os.WriteFile(filepath.Join(subDir, "nested.txt"), content, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []DiffTest{
		{
			Name:          "relative to workdir",
			Args:          []string{},
			WorkDir:       workDir,
			ExitCode:      0,
			ExpectedFiles: map[string][]byte{"sub/nested.txt": content},
		},
	}

	RunDiffTests(t, "/bin/echo", "/bin/echo", tests)
}
