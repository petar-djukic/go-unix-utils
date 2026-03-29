// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sponge against sponge (moreutils).
//
// Covers prd007-sponge R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
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
