// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for DiffTest ExpectedFiles field (prd001-testutils R5.1–R5.2).

package testutils

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDiffTest_ExpectedFilesFieldExists(t *testing.T) {
	t.Parallel()

	// R5.1: DiffTest.ExpectedFiles is a map[string][]byte.
	dt := DiffTest{
		Name: "has_expected_files",
		ExpectedFiles: map[string][]byte{
			"output.txt": []byte("content"),
		},
	}

	if dt.ExpectedFiles == nil {
		t.Fatal("ExpectedFiles should not be nil when set")
	}
	if got := dt.ExpectedFiles["output.txt"]; string(got) != "content" {
		t.Errorf("ExpectedFiles[\"output.txt\"] = %q, want %q", got, "content")
	}
}

func TestDiffTest_ExpectedFilesNilDefault(t *testing.T) {
	t.Parallel()

	// R5.1: when ExpectedFiles is nil, no file checks occur.
	dt := DiffTest{
		Name: "no_files",
	}

	if dt.ExpectedFiles != nil {
		t.Error("ExpectedFiles should be nil by default")
	}
}

func TestDiffTest_ExpectedFilesMultipleEntries(t *testing.T) {
	t.Parallel()

	// R5.1: multiple file expectations can be specified.
	dt := DiffTest{
		Name: "multi",
		ExpectedFiles: map[string][]byte{
			"a.txt": []byte("alpha"),
			"b.txt": []byte("beta"),
			"c.txt": []byte("gamma"),
		},
	}

	if len(dt.ExpectedFiles) != 3 {
		t.Errorf("ExpectedFiles has %d entries, want 3", len(dt.ExpectedFiles))
	}
}

func TestDiffTest_ExpectedFilesEmptyValue(t *testing.T) {
	t.Parallel()

	// R5.1: empty byte slices are valid expected content (empty files).
	dt := DiffTest{
		Name: "empty_content",
		ExpectedFiles: map[string][]byte{
			"empty.txt": {},
		},
	}

	if !bytes.Equal(dt.ExpectedFiles["empty.txt"], []byte{}) {
		t.Error("ExpectedFiles should accept empty byte slice as value")
	}
}

func TestRunDiffTests_ExpectedFiles_RelativePathResolution(t *testing.T) {
	t.Parallel()

	// R5.1, R5.2: relative paths in ExpectedFiles are resolved against WorkDir.
	source := `package main

import "os"

func main() {
	os.WriteFile("resolved.txt", []byte("found"), 0o644)
}
`
	bin := buildMockBinary(t, source)
	workDir := t.TempDir()

	tests := []DiffTest{
		{
			Name:    "relative_resolution",
			WorkDir: workDir,
			ExpectedFiles: map[string][]byte{
				"resolved.txt": []byte("found"),
			},
		},
	}

	RunDiffTests(t, bin, bin, tests)

	// R5.2: verify the file actually exists at the expected location.
	content, err := os.ReadFile(filepath.Join(workDir, "resolved.txt"))
	if err != nil {
		t.Fatalf("file should exist at workDir/resolved.txt: %v", err)
	}
	if !bytes.Equal(content, []byte("found")) {
		t.Errorf("file content = %q, want %q", content, "found")
	}
}

func TestRunDiffTests_ExpectedFiles_BinaryContent(t *testing.T) {
	t.Parallel()

	// R5.2: byte-for-byte comparison works with binary content.
	source := `package main

import "os"

func main() {
	os.WriteFile("binary.dat", []byte{0x00, 0x01, 0xFF, 0xFE}, 0o644)
}
`
	bin := buildMockBinary(t, source)
	workDir := t.TempDir()

	tests := []DiffTest{
		{
			Name:    "binary_file_content",
			WorkDir: workDir,
			ExpectedFiles: map[string][]byte{
				"binary.dat": {0x00, 0x01, 0xFF, 0xFE},
			},
		},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_ExpectedFiles_LargeContent(t *testing.T) {
	t.Parallel()

	// R5.2: byte-for-byte comparison works with larger content.
	largeContent := bytes.Repeat([]byte("line of text\n"), 1000)
	source := `package main

import (
	"bytes"
	"os"
)

func main() {
	content := bytes.Repeat([]byte("line of text\n"), 1000)
	os.WriteFile("large.txt", content, 0o644)
}
`
	bin := buildMockBinary(t, source)
	workDir := t.TempDir()

	tests := []DiffTest{
		{
			Name:    "large_file",
			WorkDir: workDir,
			ExpectedFiles: map[string][]byte{
				"large.txt": largeContent,
			},
		},
	}

	RunDiffTests(t, bin, bin, tests)
}
