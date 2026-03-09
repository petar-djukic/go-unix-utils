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

// refBinaryName is the Homebrew GNU cat reference binary.
const refBinaryName = "gcat"

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// cat_default_passthrough: no flags, text passes through verbatim. R1.1, R1.5.
	t.Run("cat_default_passthrough", func(t *testing.T) {
		dir := t.TempDir()
		file1 := filepath.Join(dir, "file1.txt")
		writeTestFile(t, file1, "hello\nworld\n")

		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "text_file",
				Args:    []string{file1},
				WorkDir: dir,
			},
		})
	})

	// cat_binary_passthrough: no flags, all 256 byte values pass through. R1.4.
	t.Run("cat_binary_passthrough", func(t *testing.T) {
		allBytes := make([]byte, 256)
		for i := range allBytes {
			allBytes[i] = byte(i)
		}
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "all_256_bytes",
				Args:  []string{},
				Stdin: allBytes,
			},
		})
	})

	// cat_stdin_no_args: read from stdin when no file arguments. R1.2.
	t.Run("cat_stdin_no_args", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "stdin_no_args",
				Args:  []string{},
				Stdin: []byte("from stdin\n"),
			},
		})
	})

	// cat_stdin_dash: "-" as filename reads stdin. R1.2.
	t.Run("cat_stdin_dash", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "dash_reads_stdin",
				Args:  []string{"-"},
				Stdin: []byte("from stdin\n"),
			},
		})
	})

	// cat_multiple_files: two files concatenated. R1.1, R1.3.
	t.Run("cat_multiple_files", func(t *testing.T) {
		dir := t.TempDir()
		file1 := filepath.Join(dir, "file1.txt")
		file2 := filepath.Join(dir, "file2.txt")
		writeTestFile(t, file1, "aaa\n")
		writeTestFile(t, file2, "bbb\n")

		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "two_files",
				Args:    []string{file1, file2},
				WorkDir: dir,
			},
		})
	})

	// cat_line_numbering_n: -n prepends numbers to all lines. R2.1.
	t.Run("cat_line_numbering_n", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "number_all",
				Args:  []string{"-n"},
				Stdin: []byte("alpha\n\nbeta\n"),
			},
		})
	})

	// cat_line_numbering_b: -b numbers non-blank lines only. R2.2, R2.4.
	t.Run("cat_line_numbering_b", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "number_nonblank",
				Args:  []string{"-b"},
				Stdin: []byte("first\n\n\nsecond\n"),
			},
		})
	})

	// cat_b_overrides_n: -b and -n together, -b takes precedence. R2.3.
	t.Run("cat_b_overrides_n", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "b_overrides_n",
				Args:  []string{"-b", "-n"},
				Stdin: []byte("first\n\nsecond\n"),
			},
		})
	})

	// cat_squeeze_blanks: -s collapses consecutive blank lines. R3.1.
	t.Run("cat_squeeze_blanks", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "squeeze",
				Args:  []string{"-s"},
				Stdin: []byte("a\n\n\n\nb\n"),
			},
		})
	})

	// cat_squeeze_across_files: -s applies across file boundaries. R3.2.
	t.Run("cat_squeeze_across_files", func(t *testing.T) {
		dir := t.TempDir()
		file1 := filepath.Join(dir, "f1.txt")
		file2 := filepath.Join(dir, "f2.txt")
		writeTestFile(t, file1, "a\n\n")
		writeTestFile(t, file2, "\nb\n")

		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "squeeze_across",
				Args:    []string{"-s", file1, file2},
				WorkDir: dir,
			},
		})
	})

	// cat_combined_ns: -n -s combined; squeeze before numbering. R3.3, R4.9.
	t.Run("cat_combined_ns", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "n_and_s",
				Args:  []string{"-n", "-s"},
				Stdin: []byte("a\n\n\n\nb\n"),
			},
		})
	})

	// cat_show_nonprinting: -v displays caret and M- notation. R4.1, R4.2.
	t.Run("cat_show_nonprinting", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "nonprinting",
				Args:  []string{"-v"},
				Stdin: []byte{0x01, 0x09, 0x1b, 0x7f, 0x80, 0xff},
			},
		})
	})

	// cat_show_ends: -E appends $ before newline. R4.3.
	t.Run("cat_show_ends", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "show_ends",
				Args:  []string{"-E"},
				Stdin: []byte("line one\nline two\n"),
			},
		})
	})

	// cat_show_tabs: -T displays tabs as ^I. R4.4.
	t.Run("cat_show_tabs", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "show_tabs",
				Args:  []string{"-T"},
				Stdin: []byte("col1\tcol2\tcol3\n"),
			},
		})
	})

	// cat_show_all: -A = -vET. R4.5.
	t.Run("cat_show_all", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "show_all",
				Args:  []string{"-A"},
				Stdin: []byte{0x01, 0x09, 'h', 'e', 'l', 'l', 'o', 0x0a},
			},
		})
	})

	// cat_flag_e: -e = -vE. R4.6.
	t.Run("cat_flag_e", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "flag_e",
				Args:  []string{"-e"},
				Stdin: []byte{0x01, 'h', 'e', 'l', 'l', 'o', 0x0a},
			},
		})
	})

	// cat_flag_t: -t = -vT. R4.7.
	t.Run("cat_flag_t", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "flag_t",
				Args:  []string{"-t"},
				Stdin: []byte{0x01, 0x09, 'h', 'e', 'l', 'l', 'o', 0x0a},
			},
		})
	})

	// cat_flag_u: -u accepted, no effect. R4.8.
	t.Run("cat_flag_u", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "flag_u",
				Args:  []string{"-u"},
				Stdin: []byte("test\n"),
			},
		})
	})

	// cat_missing_file: non-existent file errors, exits 1, continues. R5.2.
	t.Run("cat_missing_file", func(t *testing.T) {
		dir := t.TempDir()
		realFile := filepath.Join(dir, "real.txt")
		writeTestFile(t, realFile, "data\n")
		nonexistent := filepath.Join(dir, "nonexistent.txt")

		// Normalize stderr: gcat prints "gcat:" while our binary prints "cat:".
		stderrNorm := func(data []byte) []byte {
			data = bytes.ReplaceAll(data, []byte("gcat:"), []byte("cat:"))
			// GNU cat uses strerror() which capitalizes; Go's os package lowercases.
			data = bytes.ReplaceAll(data, []byte("No such file or directory"), []byte("no such file or directory"))
			return data
		}

		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "missing_then_real",
				Args:      []string{nonexistent, realFile},
				WorkDir:   dir,
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{stderrNorm},
			},
		})
	})

	// cat_combined_bsA: multiple flags combined. R4.9.
	t.Run("cat_combined_bsA", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "b_s_A_combined",
				Args:  []string{"-bsA"},
				Stdin: []byte("first\n\n\n\tsecond\n\n\nthird\n"),
			},
		})
	})

	// cat_nonprinting_full_range: verify all non-printing bytes. R4.1.
	t.Run("cat_nonprinting_full_range", func(t *testing.T) {
		// Build input with all byte values except 0x0a (newline) to test on one line.
		input := make([]byte, 0, 256)
		for i := range 256 {
			if i == 0x0a {
				continue
			}
			input = append(input, byte(i))
		}
		input = append(input, '\n')

		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "full_byte_range_v",
				Args:  []string{"-v"},
				Stdin: input,
			},
		})
	})

	// cat_empty_stdin: empty input produces no output. R1.1.
	t.Run("cat_empty_stdin", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "empty",
				Args:  []string{},
				Stdin: []byte{},
			},
		})
	})

	// cat_no_trailing_newline: input without trailing newline. R1.5.
	t.Run("cat_no_trailing_newline", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "no_trailing_newline_n",
				Args:  []string{"-n"},
				Stdin: []byte("hello"),
			},
		})
	})
}

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
}
