// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests: prd007-sponge R3.3, R4.1, R4.2, R4.3, R5.1, R5.2, R5.3, R5.4
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiffPassthrough exercises passthrough mode (no output filename) against
// the gsponge reference binary, covering R4.1 (no filename → stdout), R4.2
// (large input that would trigger temp file spill), and R4.3 (small in-memory
// input written directly to stdout).
func TestDiffPassthrough(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsponge")
	if err != nil {
		t.Skipf("reference binary gsponge not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R4.1, R4.3: passthrough mode with small (in-memory) stdin.
			Name:  "passthrough_small",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1, R4.2: passthrough mode with large stdin (1 MB+, forces temp file spill path).
			Name:  "passthrough_large",
			Args:  []string{},
			Stdin: bytes.Repeat([]byte("x"), 1024*1024+1),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1, R4.3: passthrough mode with empty stdin produces empty stdout.
			Name:  "passthrough_empty",
			Args:  []string{},
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1, R4.3: passthrough mode with multi-line stdin.
			Name:  "passthrough_multiline",
			Args:  []string{},
			Stdin: []byte("line1\nline2\nline3\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffAppendMode exercises append mode (-a flag) against the gsponge
// reference binary, verifying R3.3: the prepend operation copies the original
// file content into the temp file first, then appends the stdin buffer.
func TestDiffAppendMode(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsponge")
	if err != nil {
		t.Skipf("reference binary gsponge not in PATH: %v", err)
	}

	cases := []struct {
		name     string
		original []byte
		stdin    []byte
	}{
		{
			// R3.3: original file content prepended before stdin content.
			name:     "append_prepends_original_content",
			original: []byte("original content\n"),
			stdin:    []byte("stdin content\n"),
		},
		{
			// R3.3: multi-line original file content fully preserved before stdin.
			name:     "append_multiline_original",
			original: []byte("line1\nline2\nline3\n"),
			stdin:    []byte("appended\n"),
		},
		{
			// R3.3: append to file with binary content is handled correctly.
			name:     "append_binary_original",
			original: []byte{0x01, 0x02, 0x03},
			stdin:    []byte{0x04, 0x05},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refContent := runSpongeAppend(t, refBin, tc.original, tc.stdin)
			goContent := runSpongeAppend(t, goBin, tc.original, tc.stdin)
			if !bytes.Equal(refContent, goContent) {
				t.Errorf("file content mismatch for %q:\nref: %q\ngot: %q", tc.name, refContent, goContent)
			}
		})
	}
}

// runSpongeAppend writes originalContent to a temp directory, runs binary with
// -a against that file, and returns the resulting file contents. Used for
// differential testing of append mode (R3.3).
func runSpongeAppend(t *testing.T, binary string, originalContent, stdin []byte) []byte {
	t.Helper()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(outFile, originalContent, 0o644); err != nil {
		t.Fatalf("writing original file: %v", err)
	}
	cmd := exec.Command(binary, "-a", "out.txt")
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running %q: %v\noutput: %s", binary, err, out)
	}
	result, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	return result
}

// TestDiff runs the full differential test suite for sponge against gsponge,
// covering the four core scenarios defined in prd007-sponge R5.1–R5.4.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsponge")
	if err != nil {
		t.Skipf("reference binary gsponge not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R5.1: basic stdin-to-file write -- verify all buffered stdin bytes
			// are written to the named output file, producing no stdout or stderr.
			Name:    "basic_file_write",
			Args:    []string{"output.txt"},
			Stdin:   []byte("hello, sponge\n"),
			Env:     []string{"LC_ALL=C"},
			WorkDir: t.TempDir(),
		},
		{
			// R5.2: append mode (-a flag) -- verify -a writes stdin to the named
			// file (creating it when absent) identically to gsponge.
			Name:    "append_mode",
			Args:    []string{"-a", "output.txt"},
			Stdin:   []byte("appended line\n"),
			Env:     []string{"LC_ALL=C"},
			WorkDir: t.TempDir(),
		},
		{
			// R5.3: stdout mode (no file argument) -- verify stdin bytes are
			// written to stdout when no output file is specified.
			Name:  "stdout_mode",
			Stdin: []byte("output to stdout\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R5.4: empty stdin, file mode -- verify zero-byte input produces
			// an empty output file, no stdout, and exit 0.
			Name:    "empty_stdin_file",
			Args:    []string{"output.txt"},
			Stdin:   []byte{},
			Env:     []string{"LC_ALL=C"},
			WorkDir: t.TempDir(),
		},
		{
			// R5.4: empty stdin, stdout mode -- verify zero-byte input produces
			// empty stdout and exit 0 when no file argument is given.
			Name:  "empty_stdin_stdout",
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
