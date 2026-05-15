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
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:  "R1.1_default_numbering",
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.2_empty_lines_not_numbered",
			Stdin: []byte("a\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.2_all_empty_lines",
			Stdin: []byte("\n\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.1_single_line",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.2_mixed_empty_and_content",
			Stdin: []byte("\na\n\n\nb\n\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "R1.1_trailing_no_newline",
			Stdin: []byte("a\nb"),
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffFiles(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnl")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	writeFixture(t, dir, "f1.txt", "a\nb\n")
	writeFixture(t, dir, "f2.txt", "c\nd\n")

	tests := []testutils.DiffTest{
		{
			Name:    "R1.4_continuous_across_files",
			Args:    []string{"f1.txt", "f2.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		{
			Name:    "R1.3_single_named_file",
			Args:    []string{"f1.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
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
		t.Fatal("nl timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
