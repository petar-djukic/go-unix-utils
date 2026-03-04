// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cat core I/O, error handling, line numbering,
// squeeze, end markers, cross-boundary squeeze, and non-printing display.
//
// Implements prd006-cat R1.1, R1.2, R2.1, R2.3, R3.1, R3.2, R4.1, R4.2,
// R4.3, R4.4, R5.2, R5.3.
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

// TestCatCrossBoundarySqueeze verifies R3.2: when -s is active, squeeze state
// (prevBlank) persists across file boundaries. Consecutive blank lines spanning
// two files are collapsed to a single blank line.
func TestCatCrossBoundarySqueeze(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// File A ends with consecutive blank lines; file B starts with blank lines.
	writeTestFile(t, dir, "a.txt", "content-a\n\n\n")
	writeTestFile(t, dir, "b.txt", "\n\ncontent-b\n")
	// File C ends with a single blank line; file D starts with one blank line.
	writeTestFile(t, dir, "c.txt", "content-c\n\n")
	writeTestFile(t, dir, "d.txt", "\ncontent-d\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "squeeze-across-files-multiple-blanks",
			Args:     []string{"-s", "a.txt", "b.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "squeeze-across-files-single-blanks",
			Args:     []string{"-s", "c.txt", "d.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestCatNonPrintingControl verifies R4.1 for control characters: with -v,
// bytes 0x01-0x1F (excluding 0x09 tab and 0x0A newline) are displayed as
// ^A through ^_ caret notation, and byte 0x7F is displayed as ^?.
func TestCatNonPrintingControl(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// 0x01 (^A), 0x02 (^B), 0x1B (^[), 0x1F (^_)
	writeTestFile(t, dir, "control-low.txt", "\x01\x02\x1b\x1f\n")
	// 0x7F (^?)
	writeTestFile(t, dir, "del.txt", "\x7f\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "control-chars-caret-notation",
			Args:     []string{"-v", "control-low.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "del-char-caret-question",
			Args:     []string{"-v", "del.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestCatNonPrintingHighByte verifies R4.1 for high bytes: with -v, bytes in
// the range 0x80-0xFF are displayed with M- prefix notation. Control-range
// high bytes (0x80-0x9F) become M-^@ through M-^_, printable-range high bytes
// (0xA0-0xFE) become M- followed by the character, and 0xFF becomes M-^?.
func TestCatNonPrintingHighByte(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// 0x80 (M-^@), 0x9F (M-^_)
	writeTestFile(t, dir, "high-control.txt", "\x80\x9f\n")
	// 0xA0 (M- ), 0xC0 (M-@), 0xFE (M-~)
	writeTestFile(t, dir, "high-printable.txt", "\xa0\xc0\xfe\n")
	// 0xFF (M-^?)
	writeTestFile(t, dir, "high-ff.txt", "\xff\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "high-control-m-caret",
			Args:     []string{"-v", "high-control.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "high-printable-m-prefix",
			Args:     []string{"-v", "high-printable.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "high-ff-m-caret-question",
			Args:     []string{"-v", "high-ff.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestCatTabExemptFromV verifies R4.2: when only -v is given without -T,
// tab characters (0x09) pass through unchanged in the output.
func TestCatTabExemptFromV(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "tabs.txt", "before\tafter\n\t\tindented\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "v-preserves-tabs",
			Args:     []string{"-v", "tabs.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}

// TestCatShowTabs verifies R4.4: when -T is active, tab characters are
// displayed as ^I. Tests both -T alone and -T combined with -v.
func TestCatShowTabs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTestFile(t, dir, "tabs.txt", "col1\tcol2\n\tindented\n")

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "show-tabs-standalone",
			Args:     []string{"-T", "tabs.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "show-tabs-with-v",
			Args:     []string{"-T", "-v", "tabs.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	})
}
