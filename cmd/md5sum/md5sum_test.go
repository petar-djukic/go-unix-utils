// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmd5sum")
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

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		// R1.2: stdin with no arguments
		{
			Name:  "stdin-no-args",
			Stdin: []byte("abc"),
		},
		// R1.2: stdin with explicit "-"
		{
			Name:  "stdin-dash",
			Args:  []string{"-"},
			Stdin: []byte("abc"),
		},
		// R1.1: single file, text mode (default)
		{
			Name: "single-file-text",
			Args: []string{helloFile},
		},
		// R1.1: single file, binary mode
		{
			Name: "single-file-binary",
			Args: []string{"-b", helloFile},
		},
		// R1.3: BSD tag format
		{
			Name: "tag-format",
			Args: []string{"--tag", helloFile},
		},
		// R1.3: tag with binary flag (mode flag has no effect on tag)
		{
			Name: "tag-with-binary",
			Args: []string{"--tag", "-b", helloFile},
		},
		// R1.1: multiple files
		{
			Name: "multiple-files",
			Args: []string{helloFile, emptyFile},
		},
		// R1.2: empty stdin
		{
			Name:  "empty-stdin",
			Stdin: []byte{},
		},
		// R1.1: empty file
		{
			Name: "empty-file",
			Args: []string{emptyFile},
		},
		// R1.4: missing file exits 1, prints error to stderr
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
		// R1.1: text mode explicit flag
		{
			Name: "explicit-text-mode",
			Args: []string{"-t", helloFile},
		},
		// R1.2: stdin with newline-terminated content
		{
			Name:  "stdin-with-newline",
			Stdin: []byte("hello world\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
