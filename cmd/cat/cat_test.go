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
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skip("reference binary gcat not found")
	}
	tests := []testutils.DiffTest{
		{Name: "default_stdin", Stdin: []byte("hello\nworld\n")},
		{Name: "binary_passthrough", Stdin: allBytes()},
		{Name: "stdin_dash", Args: []string{"-"}, Stdin: []byte("from stdin\n")},
		{Name: "number_all_n", Args: []string{"-n"}, Stdin: []byte("alpha\n\nbeta\n")},
		{Name: "number_nonblank_b", Args: []string{"-b"}, Stdin: []byte("first\n\n\nsecond\n")},
		{Name: "squeeze_s", Args: []string{"-s"}, Stdin: []byte("a\n\n\n\nb\n")},
		{Name: "combined_ns", Args: []string{"-n", "-s"}, Stdin: []byte("a\n\n\n\nb\n")},
		{Name: "show_nonprinting_v", Args: []string{"-v"}, Stdin: []byte{0x01, 0x09, 0x1b, 0x7f, 0x80, 0xff}},
		{Name: "show_ends_E", Args: []string{"-E"}, Stdin: []byte("line one\nline two\n")},
		{Name: "show_tabs_T", Args: []string{"-T"}, Stdin: []byte("col1\tcol2\tcol3\n")},
		{Name: "show_all_A", Args: []string{"-A"}, Stdin: []byte{0x01, '\t', 'h', 'e', 'l', 'l', 'o', '\n'}},
		{Name: "flag_e", Args: []string{"-e"}, Stdin: []byte{0x01, 'h', 'e', 'l', 'l', 'o', '\n'}},
		{Name: "flag_t", Args: []string{"-t"}, Stdin: []byte{0x01, '\t', 'h', 'e', 'l', 'l', 'o', '\n'}},
		{Name: "flag_u", Args: []string{"-u"}, Stdin: []byte("test\n")},
		{Name: "combined_bsA", Args: []string{"-b", "-s", "-A"}, Stdin: []byte("line\n\n\n\nend\n")},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffFiles(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skip("reference binary gcat not found")
	}
	dir := t.TempDir()
	writeFixture(t, dir, "file1.txt", "aaa\n")
	writeFixture(t, dir, "file2.txt", "bbb\n")
	tests := []testutils.DiffTest{
		{Name: "multi_file", Args: []string{"file1.txt", "file2.txt"}, WorkDir: dir},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestMissingFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	writeFixture(t, dir, "real.txt", "data\n")
	cmd := exec.Command(goBin, "nonexistent.txt", "real.txt")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit 1, got: %v", err)
	}
	if stdout.String() != "data\n" {
		t.Fatalf("expected stdout %q, got %q", "data\n", stdout.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("nonexistent.txt")) {
		t.Fatalf("stderr should mention nonexistent.txt, got %q", stderr.String())
	}
}

// R5.4: cat must exit 0 when stdout is closed by a downstream consumer.
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
		t.Fatal("cat timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

func allBytes() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
