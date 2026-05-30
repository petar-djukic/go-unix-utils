// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var discardStderr = testutils.NormalizeFunc(func([]byte) []byte { return nil })

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpr")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	writeFile(t, dir, "three.txt", "line1\nline2\nline3\n")
	writeFile(t, dir, "many.txt", seqLines(120))

	tests := []testutils.DiffTest{
		{
			Name:  "r2_3_stdin_no_file",
			Args:  []string{"-t"},
			Stdin: []byte("hello\nworld\n"),
		},
		{
			Name:  "r2_3_stdin_dash",
			Args:  []string{"-t", "-"},
			Stdin: []byte("hello\nworld\n"),
		},
		{
			Name:    "r2_2_omit_header_short",
			Args:    []string{"-t", "three.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_2_omit_header_long",
			Args:    []string{"--omit-header", "three.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_2_omit_pagination_short",
			Args:    []string{"-T", "three.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_2_omit_pagination_long",
			Args:    []string{"--omit-pagination", "three.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_2_omit_header_many_lines",
			Args:    []string{"-t", "many.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_2_omit_pagination_many_lines",
			Args:    []string{"-T", "many.txt"},
			WorkDir: dir,
		},
		{
			Name:  "r2_1_length_implies_t",
			Args:  []string{"-l", "8"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			Name:    "r2_1_length_custom",
			Args:    []string{"-l", "20", "three.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_1_length_long_flag",
			Args:    []string{"--length=20", "three.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_1_length_many_lines",
			Args:    []string{"-l", "20", "many.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_1_header_custom",
			Args:    []string{"-h", "CUSTOM", "three.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_1_header_long_flag",
			Args:    []string{"--header=MY HEADER", "three.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_1_header_empty",
			Args:    []string{"-h", "", "three.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_1_length_with_header",
			Args:    []string{"-l", "20", "-h", "CUSTOM", "three.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_1_length_with_omit",
			Args:    []string{"-l", "20", "-t", "many.txt"},
			WorkDir: dir,
		},
		{
			Name:    "r2_3_file_arg",
			Args:    []string{"-t", "three.txt"},
			WorkDir: dir,
		},
		{
			Name:      "r5_1_nonexistent_file",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		{
			Name:      "r5_1_success_exit_code",
			Args:      []string{"-t", "three.txt"},
			WorkDir:   dir,
			ExitCode:  0,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.dat")
	data := make([]byte, 0, 500000*2)
	for range 500000 {
		data = append(data, 'x', '\n')
	}
	if err := os.WriteFile(largePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, "-t", largePath)
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
		t.Fatal("pr timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func seqLines(n int) string {
	buf := make([]byte, 0, n*4)
	for i := 1; i <= n; i++ {
		buf = append(buf, strconv.Itoa(i)...)
		buf = append(buf, '\n')
	}
	return string(buf)
}
