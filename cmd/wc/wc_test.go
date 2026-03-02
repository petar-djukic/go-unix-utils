// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/wc comparing Go binary against GNU gwc reference.
//
// Implements: prd005-wc R1, R2, R3, R4, R6
// Traces: test-rel01.0, rel01.0-uc001-wc
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinaryPath is the path to the compiled Go wc binary.
var goBinaryPath string

// refBinaryPath is the path to the GNU reference binary (gwc).
var refBinaryPath string

func TestMain(m *testing.M) {
	// Locate GNU reference binary.
	ref, err := exec.LookPath("gwc")
	if err != nil {
		fmt.Println("gwc not found; skipping wc differential tests")
		os.Exit(0)
	}
	refBinaryPath = ref

	// Build the Go wc binary to a temp directory.
	tmpDir, err := os.MkdirTemp("", "wc-test-*")
	if err != nil {
		panic("creating temp dir: " + err.Error())
	}

	goBinaryPath = filepath.Join(tmpDir, "wc")
	cmd := exec.Command("go", "build", "-o", goBinaryPath, ".")
	cmd.Dir = filepath.Join(".")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tmpDir)
		panic(fmt.Sprintf("building wc binary: %v\n%s", err, out))
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

// --- Default counting mode (prd005-wc R1.1, R1.2, R1.3) ---

func TestDefault_Stdin(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "default-three-lines-stdin",
			Stdin: []byte("foo\nbar baz\nqux\n"),
		},
		{
			Name:  "default-single-line-stdin",
			Stdin: []byte("hello world\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestDefault_File(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "input.txt", "foo\nbar baz\nqux\n")

	tests := []testutils.DiffTest{
		{
			Name:    "default-file",
			Args:    []string{"input.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Individual flags (prd005-wc R2.1-R2.5) ---

func TestFlag_Lines(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lines.txt", "one\ntwo\nthree\n")

	tests := []testutils.DiffTest{
		{
			Name:  "lines-stdin",
			Args:  []string{"-l"},
			Stdin: []byte("one\ntwo\nthree\n"),
		},
		{
			Name:    "lines-file",
			Args:    []string{"-l", "lines.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestFlag_Words(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "words.txt", "hello world\ngoodbye  cruel   world\n")

	tests := []testutils.DiffTest{
		{
			Name:  "words-stdin",
			Args:  []string{"-w"},
			Stdin: []byte("hello world\ngoodbye  cruel   world\n"),
		},
		{
			Name:    "words-file",
			Args:    []string{"-w", "words.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestFlag_Bytes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bytes.txt", "abc\n")

	tests := []testutils.DiffTest{
		{
			Name:  "bytes-stdin",
			Args:  []string{"-c"},
			Stdin: []byte("abc\n"),
		},
		{
			Name:    "bytes-file",
			Args:    []string{"-c", "bytes.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestFlag_Chars(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "chars.txt", "hello\n")

	tests := []testutils.DiffTest{
		{
			Name:  "chars-stdin",
			Args:  []string{"-m"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:    "chars-file",
			Args:    []string{"-m", "chars.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestFlag_MaxLineLength(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "maxline.txt", "short\na much longer line here\nmed\n")

	tests := []testutils.DiffTest{
		{
			Name:  "maxline-stdin",
			Args:  []string{"-L"},
			Stdin: []byte("short\na much longer line here\nmed\n"),
		},
		{
			Name:    "maxline-file",
			Args:    []string{"-L", "maxline.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Flag combination (prd005-wc R2.6) ---

func TestFlag_Combination(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "combined-lw-stdin",
			Args:  []string{"-l", "-w"},
			Stdin: []byte("one two\nthree\n"),
		},
		{
			Name:  "combined-wlc-order-stdin",
			Args:  []string{"-w", "-l", "-c"},
			Stdin: []byte("one two\nthree\n"),
		},
		{
			Name:  "combined-lwL-stdin",
			Args:  []string{"-l", "-w", "-L"},
			Stdin: []byte("short\na longer line\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Flag interaction: -m overrides -c (prd005-wc R2.3) ---

func TestFlag_CharsOverridesBytes(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "m-overrides-c",
			Args:  []string{"-c", "-m"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "c-then-m-order",
			Args:  []string{"-m", "-c"},
			Stdin: []byte("hello\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- --total modes (prd005-wc R3.3) ---

func TestTotal_Auto(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "one\n")
	writeFile(t, dir, "b.txt", "two\n")

	tests := []testutils.DiffTest{
		{
			Name:    "total-auto-single-file",
			Args:    []string{"--total=auto", "a.txt"},
			WorkDir: dir,
		},
		{
			Name:    "total-auto-multi-file",
			Args:    []string{"--total=auto", "a.txt", "b.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestTotal_Always(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "one\n")

	tests := []testutils.DiffTest{
		{
			Name:    "total-always-single-file",
			Args:    []string{"--total=always", "a.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestTotal_Only(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "a\n")
	writeFile(t, dir, "b.txt", "b\nc\n")

	tests := []testutils.DiffTest{
		{
			Name:    "total-only-multi-file",
			Args:    []string{"--total=only", "a.txt", "b.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestTotal_Never(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "x\n")
	writeFile(t, dir, "b.txt", "y\n")

	tests := []testutils.DiffTest{
		{
			Name:    "total-never-multi-file",
			Args:    []string{"--total=never", "a.txt", "b.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Multi-file output and column alignment (prd005-wc R3.1, R3.2) ---

func TestMultiFile_Alignment(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file1.txt", "hello\nworld\n")
	writeFile(t, dir, "file2.txt", "foo bar baz\n")

	tests := []testutils.DiffTest{
		{
			Name:    "multi-file-default",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dir,
		},
		{
			Name:    "multi-file-lines-words",
			Args:    []string{"-l", "-w", "file1.txt", "file2.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Error paths (prd005-wc R6.2) ---

func TestError_MissingFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "real.txt", "data\n")

	tests := []testutils.DiffTest{
		{
			Name:    "missing-file-with-valid",
			Args:    []string{"nonexistent.txt", "real.txt"},
			WorkDir: dir,
		},
		{
			Name:    "missing-file-only",
			Args:    []string{"nonexistent.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Stdin via "-" argument (prd005-wc R4.1) ---

func TestStdin_DashArgument(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "stdin-dash",
			Args:  []string{"-"},
			Stdin: []byte("stdin content\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Empty input (prd005-wc R4.3) ---

func TestEmpty_Input(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "empty.txt", "")

	tests := []testutils.DiffTest{
		{
			Name:  "empty-stdin",
			Stdin: []byte(""),
		},
		{
			Name:    "empty-file",
			Args:    []string{"empty.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Binary input (prd005-wc R4.2) ---

func TestBinary_Input(t *testing.T) {
	// Construct input with bytes > 127 and embedded NUL bytes.
	binaryData := []byte{0x00, 0xFF, 0x80, 0x0A, 0xFE, 0x7F, 0x01, 0x0A}

	tests := []testutils.DiffTest{
		{
			Name:  "binary-stdin",
			Stdin: binaryData,
		},
		{
			Name:  "binary-stdin-with-flags",
			Args:  []string{"-c", "-l", "-w"},
			Stdin: binaryData,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- --files0-from (prd005-wc R4.4) ---

func TestFiles0From(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "alpha\n")
	writeFile(t, dir, "b.txt", "bravo\ncharlie\n")

	// Create a NUL-delimited file list.
	fileList := "a.txt\x00b.txt\x00"
	writeFile(t, dir, "filelist.txt", fileList)

	tests := []testutils.DiffTest{
		{
			Name:    "files0-from-file",
			Args:    []string{"--files0-from=filelist.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}
