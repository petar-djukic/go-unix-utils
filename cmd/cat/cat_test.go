// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat comparing Go binary against GNU gcat reference.
//
// Implements: prd006-cat R1, R2, R3, R4, R5
// Traces: test-rel01.1, rel01.1-uc001-cat
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinaryPath is the path to the compiled Go cat binary.
var goBinaryPath string

// refBinaryPath is the path to the GNU reference binary (gcat).
var refBinaryPath string

func TestMain(m *testing.M) {
	// Locate GNU reference binary.
	ref, err := exec.LookPath("gcat")
	if err != nil {
		fmt.Println("gcat not found; skipping cat differential tests")
		os.Exit(0)
	}
	refBinaryPath = ref

	// Build the Go cat binary to a temp directory.
	tmpDir, err := os.MkdirTemp("", "cat-test-*")
	if err != nil {
		panic("creating temp dir: " + err.Error())
	}

	goBinaryPath = filepath.Join(tmpDir, "cat")
	cmd := exec.Command("go", "build", "-o", goBinaryPath, ".")
	cmd.Dir = filepath.Join(".")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tmpDir)
		panic(fmt.Sprintf("building cat binary: %v\n%s", err, out))
	}

	code := m.Run()
	os.RemoveAll(tmpDir) // best-effort cleanup
	os.Exit(code)
}

// writeFile creates a file with the given content in dir and returns the filename.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
	return name
}

// --- Default behavior, no flags (prd006-cat R1.1, R1.2, R1.3, R1.5) ---

func TestDefault_Stdin(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "default-stdin-text",
			Stdin: []byte("hello\nworld\n"),
		},
		{
			Name:  "default-stdin-single-line",
			Stdin: []byte("one line\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestDefault_File(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file1.txt", "hello\nworld\n")

	tests := []testutils.DiffTest{
		{
			Name:    "default-file",
			Args:    []string{"file1.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestDefault_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file1.txt", "aaa\n")
	writeFile(t, dir, "file2.txt", "bbb\n")

	tests := []testutils.DiffTest{
		{
			Name:    "multiple-files-in-order",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestStdin_DashArgument(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "stdin-dash",
			Args:  []string{"-"},
			Stdin: []byte("from stdin\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Binary passthrough (prd006-cat R1.4) ---

func TestDefault_BinaryPassthrough(t *testing.T) {
	// All 256 byte values to verify no corruption.
	binaryData := make([]byte, 256)
	for i := range binaryData {
		binaryData[i] = byte(i)
	}

	tests := []testutils.DiffTest{
		{
			Name:  "binary-passthrough-all-bytes",
			Stdin: binaryData,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Line numbering -n (prd006-cat R2.1) ---

func TestFlag_NumberAll(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "number-all-stdin",
			Args:  []string{"-n"},
			Stdin: []byte("alpha\n\nbeta\n"),
		},
		{
			Name:  "number-all-with-blanks",
			Args:  []string{"-n"},
			Stdin: []byte("a\n\n\nb\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Line numbering -b (prd006-cat R2.2) ---

func TestFlag_NumberNonblank(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "number-nonblank-stdin",
			Args:  []string{"-b"},
			Stdin: []byte("first\n\n\nsecond\n"),
		},
		{
			Name:  "number-nonblank-single-blank",
			Args:  []string{"-b"},
			Stdin: []byte("one\n\ntwo\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- -b overrides -n (prd006-cat R2.3) ---

func TestFlag_NumberNonblank_OverridesNumber(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "b-overrides-n",
			Args:  []string{"-n", "-b"},
			Stdin: []byte("first\n\nsecond\n"),
		},
		{
			Name:  "b-overrides-n-reverse-order",
			Args:  []string{"-b", "-n"},
			Stdin: []byte("first\n\nsecond\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Blank-line squeezing -s (prd006-cat R3.1, R3.2) ---

func TestFlag_Squeeze(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "squeeze-consecutive-blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		{
			Name:  "squeeze-single-blank-unchanged",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\nb\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestFlag_Squeeze_AcrossFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "end-blank.txt", "aaa\n\n")
	writeFile(t, dir, "start-blank.txt", "\nbbb\n")

	tests := []testutils.DiffTest{
		{
			Name:    "squeeze-across-files",
			Args:    []string{"-s", "end-blank.txt", "start-blank.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Combined -n -s (prd006-cat R3.3, R4.9) ---

func TestFlag_Combined_NS(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "combined-ns",
			Args:  []string{"-n", "-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Combined -b -s ---

func TestFlag_Combined_BS(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "combined-bs",
			Args:  []string{"-b", "-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Show ends -E (prd006-cat R4.3) ---

func TestFlag_ShowEnds(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "show-ends-stdin",
			Args:  []string{"-E"},
			Stdin: []byte("line one\nline two\n"),
		},
		{
			Name:  "show-ends-blank-lines",
			Args:  []string{"-E"},
			Stdin: []byte("a\n\nb\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Show tabs -T (prd006-cat R4.4) ---

func TestFlag_ShowTabs(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "show-tabs-stdin",
			Args:  []string{"-T"},
			Stdin: []byte("col1\tcol2\tcol3\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Show non-printing -v (prd006-cat R4.1, R4.2) ---

func TestFlag_ShowNonprint(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "nonprint-control-chars",
			Args:  []string{"-v"},
			Stdin: []byte{0x01, 0x09, 0x1b, 0x7f, 0x80, 0xff},
		},
		{
			Name:  "nonprint-high-bytes",
			Args:  []string{"-v"},
			Stdin: []byte{0xa0, 0xfe, 0x0a},
		},
		{
			Name:  "nonprint-preserves-tab-newline",
			Args:  []string{"-v"},
			Stdin: []byte("hello\tworld\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Show all -A (prd006-cat R4.5) ---

func TestFlag_ShowAll(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "show-all-stdin",
			Args:  []string{"-A"},
			Stdin: []byte{0x01, 0x09, 'h', 'e', 'l', 'l', 'o', 0x0a},
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Combination flag -e (prd006-cat R4.6) ---

func TestFlag_ShowEndNP(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "flag-e-stdin",
			Args:  []string{"-e"},
			Stdin: []byte{0x01, 'h', 'e', 'l', 'l', 'o', 0x0a},
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Combination flag -t (prd006-cat R4.7) ---

func TestFlag_ShowTabNP(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "flag-t-stdin",
			Args:  []string{"-t"},
			Stdin: []byte{0x01, 0x09, 'h', 'e', 'l', 'l', 'o', 0x0a},
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Flag -u accepted but ignored (prd006-cat R4.8) ---

func TestFlag_Unbuffered(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "flag-u-accepted",
			Args:  []string{"-u"},
			Stdin: []byte("test\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Error handling: missing file (prd006-cat R5.2) ---

func TestError_MissingFile(t *testing.T) {
	dir := t.TempDir()

	tests := []testutils.DiffTest{
		{
			Name:    "missing-file-only",
			Args:    []string{"nonexistent.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Error handling: mixed valid and invalid files (prd006-cat R5.2) ---

func TestError_MixedValidInvalid(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "real.txt", "data\n")

	tests := []testutils.DiffTest{
		{
			Name:    "missing-file-with-valid",
			Args:    []string{"nonexistent.txt", "real.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Empty file (prd006-cat R1.1) ---

func TestEmpty_File(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "empty.txt", "")

	tests := []testutils.DiffTest{
		{
			Name:    "empty-file",
			Args:    []string{"empty.txt"},
			WorkDir: dir,
		},
		{
			Name:  "empty-stdin",
			Stdin: []byte(""),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}
