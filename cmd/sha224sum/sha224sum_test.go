// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/sha224sum against gsha224sum.
// Implements srd074-sha224sum R1.1, R1.2, R1.3, R1.4 acceptance criteria via
// testutils.RunDiffTests.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeStderr replaces the reference binary name so differential
// comparison succeeds. Handles "gsha224sum:" and full path forms.
func normalizeStderr(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("gsha224sum:"), []byte("sha224sum:"))
	idx := bytes.Index(data, []byte("/sha224sum:"))
	for idx >= 0 {
		start := bytes.LastIndex(data[:idx], []byte("\n"))
		if start == -1 {
			start = 0
		} else {
			start++
		}
		if data[start] == '/' {
			data = append(data[:start], append([]byte("sha224sum:"), data[idx+len("/sha224sum:"):]...)...)
		}
		next := bytes.Index(data[start+10:], []byte("/sha224sum:"))
		if next == -1 {
			break
		}
		idx = start + 10 + next
	}
	data = bytes.ReplaceAll(data,
		[]byte("No such file or directory"),
		[]byte("no such file or directory"))
	return data
}

// normalizeStderrHint strips the "Try '...' for more information." line
// that GNU sha224sum appends to some error messages.
func normalizeStderrHint(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var out [][]byte
	for _, l := range lines {
		if bytes.HasPrefix(l, []byte("Try '")) {
			continue
		}
		out = append(out, l)
	}
	return bytes.Join(out, []byte("\n"))
}

// openWrapRe matches Go-style "open PATH: error" wrapping inside error messages.
var openWrapRe = regexp.MustCompile(`: open [^:]+: `)

// normalizeOpenWrap removes the Go-style "open PATH:" wrapping that Go's
// os.Open includes in error messages but GNU coreutils does not.
func normalizeOpenWrap(data []byte) []byte {
	return openWrapRe.ReplaceAllFunc(data, func(match []byte) []byte {
		return []byte(": ")
	})
}

// normalizeAlgoPrefix normalizes the algorithm prefix in error messages.
// Our implementation prints "SHA224: file:" while GNU prints "sha224sum: file:".
func normalizeAlgoPrefix(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("SHA224: "), []byte("sha224sum: "))
	return data
}

// normalizeHelpVersion normalizes --help and --version output to empty
// since our output text differs from GNU's. We only verify exit code.
func normalizeHelpVersion(data []byte) []byte {
	return nil
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestDiff runs differential tests for sha224sum against gsha224sum.
// D4: uses LC_ALL=C (default in testutils.RunDiffTests).
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha224sum")
	if err != nil {
		t.Skipf("reference binary gsha224sum not in PATH: %v", err)
	}

	dir := t.TempDir()

	// Create temporary test files for hashing.
	hello := writeTestFile(t, dir, "hello.txt", "hello\n")
	empty := writeTestFile(t, dir, "empty.txt", "")
	abc := writeTestFile(t, dir, "abc.txt", "abc")
	multi1 := writeTestFile(t, dir, "multi1.txt", "first file\n")
	multi2 := writeTestFile(t, dir, "multi2.txt", "second file\n")

	stderrNorm := []testutils.NormalizeFunc{
		normalizeStderr, normalizeStderrHint,
		normalizeAlgoPrefix, normalizeOpenWrap,
	}

	helpNorm := []testutils.NormalizeFunc{normalizeHelpVersion}

	tests := []testutils.DiffTest{
		// --- R1.1: File hashing in GNU text format ---
		{
			Name: "single_file_hash",
			Args: []string{hello},
		},
		{
			Name: "empty_file",
			Args: []string{empty},
		},
		{
			Name: "file_no_trailing_newline",
			Args: []string{abc},
		},
		{
			Name: "multi_file",
			Args: []string{multi1, multi2},
		},
		// --- R1.2: Stdin hashing ---
		{
			Name:  "stdin_no_args",
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("hello\n"),
		},
		// --- R1.3: --tag BSD-style output ---
		{
			Name: "tag_mode",
			Args: []string{"--tag", hello},
		},
		{
			Name: "tag_multi_file",
			Args: []string{"--tag", multi1, multi2},
		},
		// --- R1.4: Error handling for missing files ---
		{
			Name:      "nonexistent_file",
			Args:      []string{filepath.Join(dir, "no_such_file.txt")},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			Name:      "nonexistent_among_valid",
			Args:      []string{hello, filepath.Join(dir, "missing.txt"), multi1},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// --- Binary and text mode flags ---
		{
			Name: "binary_mode",
			Args: []string{"-b", hello},
		},
		{
			Name: "binary_mode_long",
			Args: []string{"--binary", hello},
		},
		{
			Name: "text_mode_explicit",
			Args: []string{"-t", hello},
		},
		{
			Name: "tag_with_binary",
			Args: []string{"--tag", "-b", hello},
		},
		{
			Name: "multi_file_binary",
			Args: []string{"-b", multi1, multi2},
		},
		// --- --help and --version ---
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: helpNorm,
		},
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: helpNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
