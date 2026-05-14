// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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

func lookupRefBinary(t *testing.T) string {
	t.Helper()
	ref, err := exec.LookPath("sponge")
	if err != nil {
		t.Skip("reference binary sponge not found")
	}
	return ref
}

// R4.1, R4.2, R4.3: passthrough mode (no output filename).
func TestDiffPassthrough(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin := lookupRefBinary(t)
	tests := []testutils.DiffTest{
		{Name: "passthrough_small", Stdin: []byte("hello\nworld\n")},
		{Name: "passthrough_empty", Stdin: []byte{}},
		{Name: "passthrough_single_byte", Stdin: []byte("x")},
		{Name: "passthrough_no_newline", Stdin: []byte("no trailing newline")},
		{Name: "passthrough_binary", Stdin: allBytes()},
		{Name: "passthrough_large", Stdin: bytes.Repeat([]byte("abcdefghij\n"), 200000)},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// R3.3: append mode prepend operation.
func TestDiffAppend(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin := lookupRefBinary(t)

	tests := []struct {
		name    string
		initial []byte
		stdin   []byte
	}{
		{
			name:    "append_basic",
			initial: []byte("original\n"),
			stdin:   []byte("appended\n"),
		},
		{
			name:    "append_multiline",
			initial: []byte("line1\nline2\n"),
			stdin:   []byte("line3\nline4\n"),
		},
		{
			name:    "append_empty_stdin",
			initial: []byte("keep this\n"),
			stdin:   []byte{},
		},
		{
			name:    "append_empty_original",
			initial: []byte{},
			stdin:   []byte("new content\n"),
		},
		{
			name:    "append_binary_data",
			initial: allBytes(),
			stdin:   allBytes(),
		},
		{
			name:    "append_large_stdin",
			initial: []byte("prefix\n"),
			stdin:   bytes.Repeat([]byte("data-line\n"), 200000),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			compareAppendOutput(t, goBin, refBin, tc.initial, tc.stdin)
		})
	}
}

func compareAppendOutput(t *testing.T, goBin, refBin string, initial, stdin []byte) {
	t.Helper()

	goDir := t.TempDir()
	refDir := t.TempDir()
	outName := "out.txt"
	goOut := filepath.Join(goDir, outName)
	refOut := filepath.Join(refDir, outName)

	writeFile(t, goOut, initial)
	writeFile(t, refOut, initial)

	runSponge(t, goBin, []string{"-a", goOut}, stdin)
	runSponge(t, refBin, []string{"-a", refOut}, stdin)

	goContent := readFile(t, goOut)
	refContent := readFile(t, refOut)

	if !bytes.Equal(goContent, refContent) {
		t.Fatalf("file content divergence\n  ref: %q\n  go:  %q", refContent, goContent)
	}
}

func runSponge(t *testing.T, binary string, args []string, stdin []byte) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("%s exited %d: %s", binary, exitErr.ExitCode(), stderr.String())
		}
		t.Fatalf("%s failed: %v", binary, err)
	}
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return content
}

func allBytes() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

// R5.1: exit 0 on all success paths.
func TestDiffExitSuccess(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin := lookupRefBinary(t)

	tests := []testutils.DiffTest{
		{Name: "exit0_passthrough", Stdin: []byte("hello\n")},
		{Name: "exit0_passthrough_empty", Stdin: []byte{}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)

	t.Run("exit0_file_output", func(t *testing.T) {
		dir := t.TempDir()
		runSponge(t, goBin, []string{filepath.Join(dir, "go.txt")}, []byte("content\n"))
		runSponge(t, refBin, []string{filepath.Join(dir, "ref.txt")}, []byte("content\n"))
	})

	t.Run("exit0_append_new_file", func(t *testing.T) {
		dir := t.TempDir()
		runSponge(t, goBin, []string{"-a", filepath.Join(dir, "go.txt")}, []byte("data\n"))
		runSponge(t, refBin, []string{"-a", filepath.Join(dir, "ref.txt")}, []byte("data\n"))
	})
}

// R5.2: exit 1 on error conditions with stderr message.
func TestDiffExitError(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin := lookupRefBinary(t)

	tests := []struct {
		name string
		args []string
	}{
		{name: "exit1_bad_output_dir", args: []string{"/nonexistent_sponge_test_5133/file.txt"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			goExit, goStderr := runCaptureExit(t, goBin, tc.args, []byte("data"))
			refExit, refStderr := runCaptureExit(t, refBin, tc.args, []byte("data"))

			if goExit != refExit {
				t.Fatalf("exit code divergence: go=%d ref=%d\n  go stderr: %s\n  ref stderr: %s",
					goExit, refExit, goStderr, refStderr)
			}
			if goExit != 1 {
				t.Fatalf("expected exit 1, got %d", goExit)
			}
			if len(goStderr) == 0 {
				t.Fatal("go binary produced no stderr on error")
			}
			if len(refStderr) == 0 {
				t.Fatal("ref binary produced no stderr on error")
			}
		})
	}
}

// R5.3: large input completes without panic.
func TestNoPanicLargeInput(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	largeInput := bytes.Repeat([]byte("x"), 2*1024*1024)

	cmd := exec.Command(goBin)
	cmd.Stdin = bytes.NewReader(largeInput)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge failed on large input: %v\nstderr: %s", err, stderr.String())
	}
	if stdout.Len() != len(largeInput) {
		t.Fatalf("output length %d != input length %d", stdout.Len(), len(largeInput))
	}
}

// R5.4: temp files are cleaned up on error paths.
func TestTempFileCleanup(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()

	cmd := exec.Command(goBin, "/nonexistent_sponge_test_5133/file.txt")
	cmd.Stdin = bytes.NewReader([]byte("data to sponge\n"))
	cmd.Env = envWithTMPDIR(tmpDir)
	cmd.Stderr = &bytes.Buffer{}
	_ = cmd.Run()

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("read tmpdir: %v", err)
	}
	for _, e := range entries {
		t.Errorf("temp file not cleaned up: %s", e.Name())
	}
}

func runCaptureExit(t *testing.T, binary string, args []string, stdin []byte) (int, string) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stderr.String()
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), stderr.String()
	}
	t.Fatalf("failed to run %s: %v", binary, err)
	return -1, ""
}

func envWithTMPDIR(dir string) []string {
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "TMPDIR=") {
			env = append(env, e)
		}
	}
	return append(env, "TMPDIR="+dir)
}
