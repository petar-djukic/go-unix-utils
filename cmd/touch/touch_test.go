// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/touch against gtouch (GNU coreutils).
// Implements prd062-touch R1.1–R1.4 test coverage.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameNormalizer replaces the binary name/path prefix in error
// messages so that "gtouch:" and "/path/to/touch:" both become "touch:".
func binaryNameNormalizer(b []byte) []byte {
	re := regexp.MustCompile(`(?m)^(?:\S+/)?g?touch:`)
	b = re.ReplaceAll(b, []byte("touch:"))
	reTry := regexp.MustCompile(`Try '[^']*' for more information\.`)
	b = reTry.ReplaceAll(b, []byte("Try 'touch --help' for more information."))
	return b
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skipf("reference binary gtouch not in PATH: %v", err)
	}

	norm := []testutils.NormalizeFunc{binaryNameNormalizer}

	tests := []testutils.DiffTest{
		{
			// R1.2: create new file when it does not exist.
			Name: "create_new_file",
			Args: []string{"newfile"},
		},
		{
			// R1.1: update timestamps on existing file.
			Name: "update_existing_file",
			Args: []string{"existingfile"},
		},
		{
			// R1.4: multiple file arguments.
			Name: "multiple_files",
			Args: []string{"file1", "file2", "file3"},
		},
		{
			// R1.3: -c flag suppresses creation of nonexistent file.
			Name: "no_create_short_flag",
			Args: []string{"-c", "nonexistent"},
		},
		{
			// R1.3: --no-create long form.
			Name: "no_create_long_flag",
			Args: []string{"--no-create", "nonexistent"},
		},
		{
			// R1.1, R1.3: -c on existing file still updates timestamps.
			Name: "no_create_existing_file",
			Args: []string{"-c", "existingfile"},
		},
		{
			// Missing file operand.
			Name:      "missing_operand",
			Args:      []string{},
			ExitCode:  1,
			Normalize: norm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestCreateNewFile verifies R1.2: touch creates a new empty file.
func TestCreateNewFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "newfile")

	cmd := exec.Command(goBin, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("touch failed: %v\noutput: %s", err, out)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected empty file, got size %d", info.Size())
	}
}

// TestUpdateTimestamps verifies R1.1: touch updates timestamps.
func TestUpdateTimestamps(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "existing")

	// Create file with old timestamp.
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(target, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	before := time.Now().Add(-time.Second)
	cmd := exec.Command(goBin, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("touch failed: %v\noutput: %s", err, out)
	}
	after := time.Now().Add(time.Second)

	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.ModTime().Before(before) || info.ModTime().After(after) {
		t.Fatalf("mod time not updated: got %v, want between %v and %v",
			info.ModTime(), before, after)
	}
}

// TestNoCreateFlag verifies R1.3: -c suppresses file creation.
func TestNoCreateFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "nonexistent")

	cmd := exec.Command(goBin, "-c", target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("touch -c failed: %v\noutput: %s", err, out)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("file should not have been created")
	}
}

// TestMultipleFiles verifies R1.4: multiple file arguments.
func TestMultipleFiles(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	files := []string{
		filepath.Join(dir, "a"),
		filepath.Join(dir, "b"),
		filepath.Join(dir, "c"),
	}

	cmd := exec.Command(goBin, files...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("touch failed: %v\noutput: %s", err, out)
	}

	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("file %s not created: %v", f, err)
		}
	}
}
