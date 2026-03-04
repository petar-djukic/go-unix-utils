// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cat core I/O, error handling, line numbering,
// squeeze, and end markers.
//
// Implements prd006-cat R1.1, R1.2, R2.1, R2.3, R3.1, R4.3, R5.2, R5.3.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinary is the path to the compiled Go cat binary. Set by TestMain.
var goBinary string

// refBinary is the path to the GNU gcat reference binary. Set by TestMain.
var refBinary string

// TestMain builds the Go cat binary and locates the gcat reference binary.
// D2: skip all tests if gcat is not on PATH.
// D3: build Go cat binary into a temporary directory.
func TestMain(m *testing.M) {
	ref, err := exec.LookPath("gcat")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gcat not found on PATH; skipping cat differential tests")
		os.Exit(0)
	}
	refBinary = ref

	binDir, err := os.MkdirTemp("", "cat-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating bin dir: %v\n", err)
		os.Exit(1)
	}

	goBinary = filepath.Join(binDir, "cat")
	cmd := exec.Command("go", "build", "-o", goBinary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building Go cat binary: %v\n%s", err, out)
		os.RemoveAll(binDir) // best-effort cleanup
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(binDir) // best-effort cleanup
	os.Exit(code)
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
}

// normalizeProgramName replaces "gcat: " with "cat: " in output so stderr
// from the GNU reference binary and the Go binary can be compared.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gcat: "), []byte("cat: "))
}

// TestCatFileRead verifies R1.1: cat reads named files in argument order
// and writes their contents to stdout.
func TestCatFileRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "file1.txt", "hello\nworld\n")
	writeTestFile(t, dir, "file2.txt", "foo\nbar\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "single-file",
			Args:     []string{"file1.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "multi-file-argument-order",
			Args:     []string{"file1.txt", "file2.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestCatStdinRead verifies R1.2: cat reads from stdin when no file arguments
// are given and when "-" appears as a file argument.
func TestCatStdinRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "before.txt", "before\n")
	writeTestFile(t, dir, "after.txt", "after\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "stdin-no-args",
			Stdin:    []byte("from stdin\n"),
			ExitCode: 0,
		},
		{
			Name:     "stdin-dash-arg",
			Args:     []string{"-"},
			Stdin:    []byte("dash stdin\n"),
			ExitCode: 0,
		},
		{
			Name:     "stdin-dash-interspersed",
			Args:     []string{"before.txt", "-", "after.txt"},
			Stdin:    []byte("middle\n"),
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestCatErrorHandling verifies R5.2 and R5.3: cat writes error messages to
// stderr for nonexistent files, continues processing remaining arguments, and
// exits with code 1 when any file produces an error.
func TestCatErrorHandling(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "real.txt", "data\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:      "nonexistent-file",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		{
			Name:      "mixed-valid-invalid",
			Args:      []string{"nonexistent.txt", "real.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	})
}

// TestCatLineNumbering verifies R2.1: cat -n numbers all output lines with
// the format "%6d\t" prefix, including blank lines.
func TestCatLineNumbering(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "mixed.txt", "first\n\nsecond\n\n\nthird\n")
	writeTestFile(t, dir, "simple.txt", "alpha\nbeta\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "number-all-lines-with-blanks",
			Args:     []string{"-n", "mixed.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "number-across-files",
			Args:     []string{"-n", "simple.txt", "mixed.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestCatNumberNonblank verifies R2.2 and R2.3: cat -b numbers only non-blank
// lines, and -b overrides -n when both are given.
func TestCatNumberNonblank(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "blanks.txt", "line1\n\n\nline2\n\nline3\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "number-nonblank-only",
			Args:     []string{"-b", "blanks.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "b-overrides-n",
			Args:     []string{"-b", "-n", "blanks.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestCatSqueeze verifies R3.1: cat -s squeezes runs of consecutive blank
// lines down to a single blank line.
func TestCatSqueeze(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "multi-blank.txt", "top\n\n\n\nmiddle\n\n\nbottom\n")
	writeTestFile(t, dir, "double-blank.txt", "a\n\n\nb\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "squeeze-multiple-blanks",
			Args:     []string{"-s", "multi-blank.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "squeeze-two-blanks",
			Args:     []string{"-s", "double-blank.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestCatShowEnds verifies R4.3: cat -E appends a "$" character before each
// newline in the output.
func TestCatShowEnds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "lines.txt", "hello\nworld\n")
	writeTestFile(t, dir, "with-blanks.txt", "first\n\nsecond\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "show-ends-basic",
			Args:     []string{"-E", "lines.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "show-ends-with-blank-lines",
			Args:     []string{"-E", "with-blanks.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}
