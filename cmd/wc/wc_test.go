// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for wc core counting and output formatting.
//
// Implements prd005-wc R1.1, R1.2, R1.3, R1.4.
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

// goBinary is the path to the compiled Go wc binary. Set by TestMain.
var goBinary string

// refBinary is the path to the GNU gwc reference binary. Set by TestMain.
var refBinary string

// TestMain builds the Go wc binary and locates the gwc reference binary.
// D1: skip all tests if gwc is not on PATH.
// D1: build Go wc binary into a temporary directory.
func TestMain(m *testing.M) {
	ref, err := exec.LookPath("gwc")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gwc not found on PATH; skipping wc differential tests")
		os.Exit(0)
	}
	refBinary = ref

	binDir, err := os.MkdirTemp("", "wc-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating bin dir: %v\n", err)
		os.Exit(1)
	}

	goBinary = filepath.Join(binDir, "wc")
	cmd := exec.Command("go", "build", "-o", goBinary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building Go wc binary: %v\n%s", err, out)
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

// normalizeProgramName replaces "gwc: " with "wc: " in output so stderr
// from the GNU reference binary and the Go binary can be compared.
// D3: follows the pattern in cmd/cat/cat_test.go.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gwc: "), []byte("wc: "))
}

// TestWcDefaultMode verifies R1.1: when no flags are given, wc prints line
// count, word count, and byte count for each input.
func TestWcDefaultMode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "three_lines.txt", "foo\nbar baz\nqux\n")
	writeTestFile(t, dir, "empty.txt", "")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "single-file-default",
			Args:     []string{"three_lines.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "empty-file-default",
			Args:     []string{"empty.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestWcIndividualFlags verifies R1.1: individual flag selection (-l, -w, -c)
// each produces output matching gwc byte-for-byte.
func TestWcIndividualFlags(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "sample.txt", "hello world\ngoodbye  cruel   world\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "lines-only",
			Args:     []string{"-l", "sample.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "words-only",
			Args:     []string{"-w", "sample.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "bytes-only",
			Args:     []string{"-c", "sample.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestWcCombinedFlags verifies R1.1 and R2.6: combined flags produce counts
// in fixed column order (lines, words, bytes) regardless of flag order.
func TestWcCombinedFlags(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "combo.txt", "one two\nthree\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "combined-flags-lwc",
			Args:     []string{"-l", "-w", "-c", "combo.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "combined-flags-reverse-order",
			Args:     []string{"-c", "-w", "-l", "combo.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestWcStdinRead verifies R1.2: wc reads from stdin when no file arguments
// are given and when "-" appears as a file argument.
func TestWcStdinRead(t *testing.T) {
	t.Parallel()

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
			Name:     "stdin-empty",
			Stdin:    []byte(""),
			ExitCode: 0,
		},
	})
}

// TestWcOutputFormat verifies R1.3: each output line has right-justified counts
// with column widths matching gwc, followed by the filename.
func TestWcOutputFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "small.txt", "a\n")
	writeTestFile(t, dir, "larger.txt", "this file has more content for wider columns\nand a second line\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "format-single-file",
			Args:     []string{"small.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "format-two-files-alignment",
			Args:     []string{"small.txt", "larger.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestWcMultiFileTotal verifies R1.4: when two or more files are given, wc
// prints a "total" summary line after the per-file lines, matching gwc.
func TestWcMultiFileTotal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "file1.txt", "hello\nworld\n")
	writeTestFile(t, dir, "file2.txt", "foo bar baz\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "two-files-total-line",
			Args:     []string{"file1.txt", "file2.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestWcMultiFileThree verifies R1.4: the total line is correct for three
// files, confirming summation across all inputs.
func TestWcMultiFileThree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "a.txt", "alpha\n")
	writeTestFile(t, dir, "b.txt", "bravo charlie\n")
	writeTestFile(t, dir, "c.txt", "delta\nepsilon\nfoxtrot\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "three-files-total-line",
			Args:     []string{"a.txt", "b.txt", "c.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestWcErrorHandling verifies R6.2: wc exits 1 for a nonexistent file,
// writes an error to stderr, and continues processing remaining arguments.
func TestWcErrorHandling(t *testing.T) {
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
