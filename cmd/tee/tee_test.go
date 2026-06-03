// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestDiffAppendLongFlag(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not found")
	}
	runAppendTestLong(t, goBin, "go_append_long")
	runAppendTestLong(t, refBin, "ref_append_long")
}

func TestDiffCombinedFlags(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not found")
	}
	input := []byte("combined\n")
	tests := []testutils.DiffTest{
		{Name: "combined_ai", Args: []string{"-ai"}, Stdin: input},
		{Name: "combined_ia", Args: []string{"-ia"}, Stdin: input},
		{Name: "separate_a_i", Args: []string{"-a", "-i"}, Stdin: input},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffIgnoreInterrupts(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not found")
	}
	input := []byte("interrupt test\n")
	tests := []testutils.DiffTest{
		{Name: "ignore_interrupts_short", Args: []string{"-i"}, Stdin: input},
		{Name: "ignore_interrupts_long", Args: []string{"--ignore-interrupts"}, Stdin: input},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffWriteOrder(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not found")
	}
	var ordered []byte
	for i := range 100 {
		ordered = fmt.Appendf(ordered, "line %03d\n", i)
	}
	tests := []testutils.DiffTest{
		{Name: "write_order", Stdin: ordered},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffMultiFileAppend(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not found")
	}
	runMultiFileAppendTest(t, goBin, "go_multi_append")
	runMultiFileAppendTest(t, refBin, "ref_multi_append")
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

func runAppendTestLong(t *testing.T, binary, label string) {
	t.Helper()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "append_long.txt")
	os.WriteFile(outFile, []byte("existing\n"), 0o644)
	cmd := exec.Command(binary, "--append", outFile)
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

func runMultiFileAppendTest(t *testing.T, binary, label string) {
	t.Helper()
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.txt")
	fileB := filepath.Join(dir, "b.txt")
	os.WriteFile(fileA, []byte("old_a\n"), 0o644)
	os.WriteFile(fileB, []byte("old_b\n"), 0o644)
	cmd := exec.Command(binary, "-a", fileA, fileB)
	cmd.Stdin = bytes.NewReader([]byte("new\n"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s: unexpected error: %v", label, err)
	}
	gotA, err := os.ReadFile(fileA)
	if err != nil {
		t.Fatalf("%s: read a.txt: %v", label, err)
	}
	gotB, err := os.ReadFile(fileB)
	if err != nil {
		t.Fatalf("%s: read b.txt: %v", label, err)
	}
	wantA := "old_a\nnew\n"
	wantB := "old_b\nnew\n"
	if string(gotA) != wantA {
		t.Fatalf("%s: a.txt\n  got:  %q\n  want: %q", label, gotA, wantA)
	}
	if string(gotB) != wantB {
		t.Fatalf("%s: b.txt\n  got:  %q\n  want: %q", label, gotB, wantB)
	}
}

func TestWriteErrorReadOnly(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	roDir := filepath.Join(dir, "readonly")
	os.Mkdir(roDir, 0o555)
	badFile := filepath.Join(roDir, "nope.txt")
	goodFile := filepath.Join(dir, "good.txt")

	goExit, goStdout, goStderr := runCapture(t, goBin, []byte("data\n"), badFile, goodFile)
	if goExit != 1 {
		t.Fatalf("expected exit 1, got %d", goExit)
	}
	if string(goStdout) != "data\n" {
		t.Fatalf("stdout: got %q, want %q", goStdout, "data\n")
	}
	got, err := os.ReadFile(goodFile)
	if err != nil {
		t.Fatalf("read good file: %v", err)
	}
	if string(got) != "data\n" {
		t.Fatalf("good file: got %q, want %q", got, "data\n")
	}
	if !strings.Contains(string(goStderr), "nope.txt") {
		t.Fatalf("stderr should mention failed file, got: %q", goStderr)
	}

	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not found")
	}
	refExit, _, _ := runCapture(t, refBin, []byte("data\n"), badFile, goodFile)
	if goExit != refExit {
		t.Fatalf("exit code divergence: go=%d ref=%d", goExit, refExit)
	}
}

func TestWriteErrorContinuesWriting(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	roDir := filepath.Join(dir, "readonly")
	os.Mkdir(roDir, 0o555)
	badFile := filepath.Join(roDir, "fail.txt")
	fileA := filepath.Join(dir, "a.txt")
	fileB := filepath.Join(dir, "b.txt")

	exit, stdout, _ := runCapture(t, goBin, []byte("hello\n"), fileA, badFile, fileB)
	if exit != 1 {
		t.Fatalf("expected exit 1, got %d", exit)
	}
	if string(stdout) != "hello\n" {
		t.Fatalf("stdout: got %q, want %q", stdout, "hello\n")
	}
	for _, p := range []string{fileA, fileB} {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if string(got) != "hello\n" {
			t.Fatalf("%s: got %q, want %q", p, got, "hello\n")
		}
	}
}

func TestBrokenStdout(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	goExit := runWithBrokenStdout(t, goBin)
	if goExit != 1 {
		t.Fatalf("expected exit 1, got %d", goExit)
	}
}

func TestExitZeroOnSuccess(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")
	cmd := exec.Command(goBin, outFile)
	cmd.Stdin = bytes.NewReader([]byte("ok\n"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected exit 0, got: %v", err)
	}
}

func TestDiffSIGINTSignal(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not found")
	}
	goStdout, goFile, goExit := runSIGINTTest(t, goBin, "go")
	refStdout, refFile, refExit := runSIGINTTest(t, refBin, "ref")
	if goExit != refExit {
		t.Fatalf("exit code: go=%d ref=%d", goExit, refExit)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Fatalf("stdout divergence\n  go:  %q\n  ref: %q", goStdout, refStdout)
	}
	if !bytes.Equal(goFile, refFile) {
		t.Fatalf("file divergence\n  go:  %q\n  ref: %q", goFile, refFile)
	}
}

func TestFileMatchesStdout(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	input := []byte("verify\nbyte-for-byte\nmatch\n")
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.txt")
	fileB := filepath.Join(dir, "b.txt")
	cmd := exec.Command(goBin, fileA, fileB)
	cmd.Stdin = bytes.NewReader(input)
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, p := range []string{fileA, fileB} {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if !bytes.Equal(got, stdout) {
			t.Fatalf("%s content != stdout\n  file:   %q\n  stdout: %q", p, got, stdout)
		}
	}
}

func runCapture(
	t *testing.T, binary string, stdin []byte, args ...string,
) (int, []byte, []byte) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.Bytes(), stderr.Bytes()
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode(), stdout.Bytes(), stderr.Bytes()
	}
	t.Fatalf("unexpected error: %v", err)
	return -1, nil, nil
}

func runWithBrokenStdout(t *testing.T, binary string) int {
	t.Helper()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary)
	cmd.Stdin = bytes.NewReader(bytes.Repeat([]byte("a\n"), 100000))
	cmd.Stdout = pw
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		t.Fatal(err)
	}
	pw.Close()
	pr.Close()
	err = cmd.Wait()
	if err == nil {
		return 0
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode()
	}
	t.Fatalf("unexpected error: %v", err)
	return -1
}

func allBytes() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func runSIGINTTest(t *testing.T, binary, label string) ([]byte, []byte, int) {
	t.Helper()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")
	cmd := exec.Command(binary, "-i", outFile)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("%s: stdin pipe: %v", label, err)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("%s: start: %v", label, err)
	}
	// Wait for the process to create the output file before proceeding.
	for range 50 {
		if _, serr := os.Stat(outFile); serr == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	stdin.Write([]byte("before\n"))
	time.Sleep(100 * time.Millisecond)
	cmd.Process.Signal(os.Interrupt)
	time.Sleep(100 * time.Millisecond)
	stdin.Write([]byte("after\n"))
	stdin.Close()
	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("%s: wait: %v", label, err)
		}
	}
	file, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("%s: read file: %v (stderr: %s)", label, err, stderr.String())
	}
	return stdout.Bytes(), file, exitCode
}
