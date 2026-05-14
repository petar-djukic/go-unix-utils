// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not found")
	}
	tests := []testutils.DiffTest{
		{Name: "passthrough_no_files", Stdin: []byte("hello\nworld\n")},
		{Name: "empty_stdin", Stdin: []byte{}},
		{Name: "large_input", Stdin: bytes.Repeat([]byte("abcdefghij\n"), 1000)},
		{Name: "no_trailing_newline", Stdin: []byte("no newline")},
		{Name: "binary_data", Stdin: allBytes()},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffSingleFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not found")
	}
	input := []byte("hello\nworld\n")
	runFileTest(t, goBin, "go_single", input, "out.txt")
	runFileTest(t, refBin, "ref_single", input, "out.txt")
}

func TestDiffMultipleFiles(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not found")
	}
	input := []byte("multi\nfile\ntest\n")
	goStdout := runMultiFileTest(t, goBin, "go_multi", input, "a.txt", "b.txt")
	refStdout := runMultiFileTest(t, refBin, "ref_multi", input, "a.txt", "b.txt")
	if !bytes.Equal(goStdout, refStdout) {
		t.Fatalf("stdout divergence\n  go:  %q\n  ref: %q", goStdout, refStdout)
	}
}

func TestDiffDashFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not found")
	}
	input := []byte("dash test\n")
	tests := []testutils.DiffTest{
		{Name: "dash_only", Args: []string{"-"}, Stdin: input},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffAppendMode(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not found")
	}
	runAppendTest(t, goBin, "go_append")
	runAppendTest(t, refBin, "ref_append")
}

func TestFileCreation(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	outFile := filepath.Join(dir, "created.txt")
	cmd := exec.Command(goBin, outFile)
	cmd.Stdin = bytes.NewReader([]byte("create me\n"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "create me\n" {
		t.Fatalf("stdout: got %q, want %q", out, "create me\n")
	}
	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if string(got) != "create me\n" {
		t.Fatalf("file content: got %q, want %q", got, "create me\n")
	}
}

func TestTruncateExisting(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	outFile := filepath.Join(dir, "existing.txt")
	os.WriteFile(outFile, []byte("old content\n"), 0o644)
	cmd := exec.Command(goBin, outFile)
	cmd.Stdin = bytes.NewReader([]byte("new\n"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "new\n" {
		t.Fatalf("file content: got %q, want %q", got, "new\n")
	}
}

func runFileTest(t *testing.T, binary, label string, input []byte, fileName string) {
	t.Helper()
	dir := t.TempDir()
	outPath := filepath.Join(dir, fileName)
	cmd := exec.Command(binary, outPath)
	cmd.Stdin = bytes.NewReader(input)
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", label, err)
	}
	if !bytes.Equal(stdout, input) {
		t.Fatalf("%s: stdout divergence\n  got:  %q\n  want: %q", label, stdout, input)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("%s: read output file: %v", label, err)
	}
	if !bytes.Equal(got, input) {
		t.Fatalf("%s: file content divergence\n  got:  %q\n  want: %q", label, got, input)
	}
}

func runMultiFileTest(
	t *testing.T, binary, label string, input []byte, names ...string,
) []byte {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for _, n := range names {
		paths = append(paths, filepath.Join(dir, n))
	}
	cmd := exec.Command(binary, paths...)
	cmd.Stdin = bytes.NewReader(input)
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", label, err)
	}
	for _, p := range paths {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: read %s: %v", label, p, err)
		}
		if !bytes.Equal(got, input) {
			t.Fatalf("%s: %s divergence\n  got:  %q\n  want: %q", label, p, got, input)
		}
	}
	return stdout
}

func runAppendTest(t *testing.T, binary, label string) {
	t.Helper()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "append.txt")
	os.WriteFile(outFile, []byte("existing\n"), 0o644)
	cmd := exec.Command(binary, "-a", outFile)
	cmd.Stdin = bytes.NewReader([]byte("appended\n"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s: unexpected error: %v", label, err)
	}
	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("%s: read file: %v", label, err)
	}
	want := "existing\nappended\n"
	if string(got) != want {
		t.Fatalf("%s: file content\n  got:  %q\n  want: %q", label, got, want)
	}
}

func allBytes() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}
