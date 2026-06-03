// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsum")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tmpDir := t.TempDir()
	helloFile := filepath.Join(tmpDir, "hello.txt")
	if err := os.WriteFile(helloFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	binaryFile := filepath.Join(tmpDir, "binary.dat")
	if err := os.WriteFile(binaryFile, []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}, 0o644); err != nil {
		t.Fatal(err)
	}
	largeFile := filepath.Join(tmpDir, "large.txt")
	if err := os.WriteFile(largeFile, bytes.Repeat([]byte("abcdefghij"), 300), 0o644); err != nil {
		t.Fatal(err)
	}

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		// R1.1: single file produces BSD checksum and block count
		{
			Name: "single-file",
			Args: []string{helloFile},
		},
		{
			Name: "empty-file",
			Args: []string{emptyFile},
		},
		{
			Name: "binary-content",
			Args: []string{binaryFile},
		},
		{
			Name: "large-file-multiple-blocks",
			Args: []string{largeFile},
		},
		// R1.2: stdin with no arguments
		{
			Name:  "stdin-no-args",
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "stdin-empty",
			Stdin: []byte{},
		},
		// R1.3: multiple files in argument order
		{
			Name: "multiple-files",
			Args: []string{helloFile, emptyFile},
		},
		{
			Name: "multiple-files-reversed",
			Args: []string{emptyFile, helloFile},
		},
		{
			Name: "three-files",
			Args: []string{helloFile, emptyFile, binaryFile},
		},
		// R1.4: missing file exits 1
		{
			Name:      "missing-file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R1.4: missing file among valid files continues processing
		{
			Name:      "missing-among-valid",
			Args:      []string{helloFile, filepath.Join(tmpDir, "nonexistent.txt"), emptyFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.1: -r selects BSD algorithm (default)
		{
			Name: "flag-r-single-file",
			Args: []string{"-r", helloFile},
		},
		{
			Name:  "flag-r-stdin",
			Args:  []string{"-r"},
			Stdin: []byte("hello\n"),
		},
		// R2.2: -s selects System V algorithm
		{
			Name: "flag-s-single-file",
			Args: []string{"-s", helloFile},
		},
		{
			Name: "flag-s-empty-file",
			Args: []string{"-s", emptyFile},
		},
		{
			Name: "flag-s-binary",
			Args: []string{"-s", binaryFile},
		},
		{
			Name: "flag-s-large-file",
			Args: []string{"-s", largeFile},
		},
		{
			Name:  "flag-s-stdin",
			Args:  []string{"-s"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "flag-s-stdin-empty",
			Args:  []string{"-s"},
			Stdin: []byte{},
		},
		{
			Name: "flag-s-multiple-files",
			Args: []string{"-s", helloFile, emptyFile},
		},
		{
			Name:      "flag-s-missing-file",
			Args:      []string{"-s", filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.dat")
	if err := os.WriteFile(largePath, bytes.Repeat([]byte("x\n"), 500000), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, largePath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		t.Fatal(err)
	}
	stdout.Close()
	err = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("sum timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}
