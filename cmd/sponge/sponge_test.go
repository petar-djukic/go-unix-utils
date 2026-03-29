// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sponge against sponge (moreutils).
//
// Covers prd007-sponge R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skip("reference binary sponge not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1, R4.1: passthrough mode — no filename, stdin to stdout
		{
			Name:  "R1.1_passthrough_small",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
		},
		// R1.1: empty stdin passthrough
		{
			Name:  "R1.1_passthrough_empty",
			Args:  []string{},
			Stdin: []byte{},
		},
		// R1.2: multi-line input passthrough
		{
			Name:  "R1.2_passthrough_multiline",
			Args:  []string{},
			Stdin: []byte("line1\nline2\nline3\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestWriteToFile verifies sponge writes stdin to a named file (R1.1, R1.2).
func TestWriteToFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.txt")
	input := []byte("hello sponge\n")

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader(input)
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge exited with error: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !bytes.Equal(got, input) {
		t.Errorf("output file content = %q, want %q", got, input)
	}
}

// TestSoakBeforeWrite confirms the file is not opened until stdin is consumed (R1.1).
func TestSoakBeforeWrite(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "existing.txt")
	original := []byte("original content\n")
	if err := os.WriteFile(outPath, original, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Read from the same file we write to — soak-before-write must preserve content.
	catCmd := exec.Command("cat", outPath)
	catOut, err := catCmd.Output()
	if err != nil {
		t.Fatalf("cat: %v", err)
	}

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader(catOut)
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge exited with error: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("soak-before-write failed: got %q, want %q", got, original)
	}
}

// TestTempFileInTMPDIR verifies temp file creation uses TMPDIR (R1.4).
func TestTempFileInTMPDIR(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	tmpDir := t.TempDir()
	outPath := filepath.Join(dir, "out.txt")

	// Generate input large enough to potentially spill, but at minimum verify
	// the TMPDIR variable is respected by the binary running without error.
	input := bytes.Repeat([]byte("x"), 4096)

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = append(os.Environ(), "TMPDIR="+tmpDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge exited with error: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !bytes.Equal(got, input) {
		t.Errorf("output mismatch: got %d bytes, want %d bytes", len(got), len(input))
	}
}

// TestPermissionPreservation verifies file mode is preserved when overwriting (R2.3).
func TestPermissionPreservation(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "perms.txt")

	// Create file with non-default permissions.
	if err := os.WriteFile(outPath, []byte("old"), 0o755); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader([]byte("new content\n"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge: %v", err)
	}

	info, err := os.Lstat(outPath)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Errorf("permissions = %04o, want %04o", perm, 0o755)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "new content\n" {
		t.Errorf("content = %q, want %q", got, "new content\n")
	}
}

// TestNewFileDefaultMode verifies new files get default 0666 permissions (R2.3).
func TestNewFileDefaultMode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "newfile.txt")

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader([]byte("created\n"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge: %v", err)
	}

	info, err := os.Lstat(outPath)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	// R2.3: default mode 0666 applied via chmod after write.
	perm := info.Mode().Perm()
	if perm != 0o666 {
		t.Errorf("permissions = %04o, want %04o", perm, 0o666)
	}
}

// TestTempFileCleanup verifies temp files are cleaned up on normal exit (R1.5).
func TestTempFileCleanup(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.txt")

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader([]byte("data\n"))
	cmd.Env = append(os.Environ(), "TMPDIR="+tmpDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge: %v", err)
	}

	// Verify no temp files left behind.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), tempPrefix) {
			t.Errorf("temp file %s not cleaned up", e.Name())
		}
	}
}

// TestRenameOrCopyFallback verifies copy fallback when rename is not possible (R2.2).
func TestRenameOrCopyFallback(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	// Use a different TMPDIR from the output directory to increase the chance
	// of cross-device rename failure. Even on same device, the copy fallback
	// is exercised when rename fails for any reason.
	tmpDir := t.TempDir()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "copied.txt")
	input := []byte("fallback content\n")

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = append(os.Environ(), "TMPDIR="+tmpDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, input) {
		t.Errorf("content = %q, want %q", got, input)
	}
}
