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
