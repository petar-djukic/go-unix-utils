// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests: prd007-sponge R1.1–R1.4 via differential testing against sponge
// (Homebrew moreutils).
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
		t.Skipf("reference binary sponge not in PATH: %v", err)
	}

	dir := t.TempDir()

	// R1.3: stdin to stdout (passthrough mode, no filename).
	t.Run("stdin_to_stdout", func(t *testing.T) {
		tests := []testutils.DiffTest{
			{
				Name:    "small_stdin_to_stdout",
				Stdin:   []byte("hello\n"),
				WorkDir: dir,
			},
			{
				Name:    "empty_stdin_to_stdout",
				Stdin:   []byte{},
				WorkDir: dir,
			},
			{
				Name:    "multiline_stdin_to_stdout",
				Stdin:   []byte("line1\nline2\nline3\n"),
				WorkDir: dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.1, R1.2: stdin to file.
	t.Run("stdin_to_file", func(t *testing.T) {
		outPath := filepath.Join(dir, "out_basic.txt")
		tests := []testutils.DiffTest{
			{
				Name:          "small_stdin_to_file",
				Args:          []string{outPath},
				Stdin:         []byte("hello\n"),
				WorkDir:       dir,
				ExpectedFiles: map[string][]byte{outPath: []byte("hello\n")},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.1: same-file pipeline (the core sponge use case).
	t.Run("same_file_pipeline", func(t *testing.T) {
		// Create a file, read it with cat, pipe through sponge back to same file.
		// This verifies sponge reads ALL stdin before opening the output.
		srcPath := filepath.Join(dir, "same_file.txt")
		content := "alpha\nbeta\ngamma\n"

		// Test with Go binary.
		if err := os.WriteFile(srcPath, []byte(content), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		goCmd := exec.Command("sh", "-c", "cat "+srcPath+" | "+goBin+" "+srcPath)
		goCmd.Dir = dir
		goOut, goErr := goCmd.CombinedOutput()
		if goErr != nil {
			t.Fatalf("go binary failed: %v\n%s", goErr, goOut)
		}
		goResult, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("read after go: %v", err)
		}

		// Test with reference binary.
		if err := os.WriteFile(srcPath, []byte(content), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		refCmd := exec.Command("sh", "-c", "cat "+srcPath+" | "+refBin+" "+srcPath)
		refCmd.Dir = dir
		refOut, refErr := refCmd.CombinedOutput()
		if refErr != nil {
			t.Fatalf("ref binary failed: %v\n%s", refErr, refOut)
		}
		refResult, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("read after ref: %v", err)
		}

		if !bytes.Equal(goResult, refResult) {
			t.Errorf("same-file pipeline divergence:\ngo:  %q\nref: %q", goResult, refResult)
		}
		if string(goResult) != content {
			t.Errorf("same-file pipeline: expected %q, got %q", content, goResult)
		}
	})

	// R1.1, R1.2: overwrite existing file.
	t.Run("overwrite_existing", func(t *testing.T) {
		outPath := filepath.Join(dir, "out_overwrite.txt")
		// Create existing file with different content.
		if err := os.WriteFile(outPath, []byte("old content\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		tests := []testutils.DiffTest{
			{
				Name:          "overwrite_existing_file",
				Args:          []string{outPath},
				Stdin:         []byte("new content\n"),
				WorkDir:       dir,
				ExpectedFiles: map[string][]byte{outPath: []byte("new content\n")},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.1: large input to verify buffering works.
	t.Run("large_input", func(t *testing.T) {
		outPath := filepath.Join(dir, "out_large.txt")
		// Generate ~100KB of data.
		var largeInput strings.Builder
		for range 10000 {
			largeInput.WriteString("this is line number for large input test\n")
		}
		data := []byte(largeInput.String())
		tests := []testutils.DiffTest{
			{
				Name:          "large_stdin_to_file",
				Args:          []string{outPath},
				Stdin:         data,
				WorkDir:       dir,
				ExpectedFiles: map[string][]byte{outPath: data},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.3: large input to stdout.
	t.Run("large_input_stdout", func(t *testing.T) {
		var largeInput strings.Builder
		for range 5000 {
			largeInput.WriteString("stdout large input line\n")
		}
		data := []byte(largeInput.String())
		tests := []testutils.DiffTest{
			{
				Name:    "large_stdin_to_stdout",
				Stdin:   data,
				WorkDir: dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}
