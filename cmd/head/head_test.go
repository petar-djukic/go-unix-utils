// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghead")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		// R4.4: default 10 lines (no flags)
		{
			Name:  "default-10-lines",
			Stdin: seq(1, 20),
		},
		// R4.4: explicit -n count
		{
			Name:  "explicit-n-5",
			Args:  []string{"-n", "5"},
			Stdin: seq(1, 20),
		},
		// R4.4: negative line count -n -5
		{
			Name:  "negative-n-5",
			Args:  []string{"-n", "-5"},
			Stdin: seq(1, 20),
		},
		// R4.4: negative byte count -c -100
		{
			Name:  "negative-c-100",
			Args:  []string{"-c", "-100"},
			Stdin: bytes512(),
		},
		// R4.4: stdin input (no file arguments)
		{
			Name:  "stdin-no-args",
			Stdin: []byte("line1\nline2\nline3\n"),
		},

		// R1.5: last line without newline is still counted
		{
			Name:  "line-no-trailing-newline",
			Args:  []string{"-n", "3"},
			Stdin: []byte("a\nb\nc"),
		},
		{
			Name:  "line-with-trailing-newline",
			Args:  []string{"-n", "3"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			Name:  "line-count-exceeds-input",
			Args:  []string{"-n", "100"},
			Stdin: []byte("one\ntwo\n"),
		},
		{
			Name:  "line-no-trailing-newline-negative",
			Args:  []string{"-n", "-1"},
			Stdin: []byte("a\nb\nc"),
		},

		// R2.1: -c NUM byte count
		{
			Name:  "bytes-5",
			Args:  []string{"-c", "5"},
			Stdin: []byte("abcdefghij"),
		},
		{
			Name:  "bytes-exceeds-input",
			Args:  []string{"-c", "100"},
			Stdin: []byte("short"),
		},
		{
			Name:  "bytes-zero",
			Args:  []string{"-c", "0"},
			Stdin: []byte("hello"),
		},

		// R2.1: -c and -n mutual exclusivity (last wins)
		{
			Name:  "bytes-after-lines",
			Args:  []string{"-n", "2", "-c", "5"},
			Stdin: []byte("abcdefghij\nklmnop\n"),
		},
		{
			Name:  "lines-after-bytes",
			Args:  []string{"-c", "5", "-n", "1"},
			Stdin: []byte("abcdefghij\nklmnop\n"),
		},

		// R2.2: negative byte count
		{
			Name:  "bytes-negative",
			Args:  []string{"-c", "-3"},
			Stdin: []byte("abcdefghij"),
		},
		{
			Name:  "bytes-negative-exceeds-input",
			Args:  []string{"-c", "-100"},
			Stdin: []byte("short"),
		},

		// R2.3: multiplier suffixes
		{
			Name:  "bytes-suffix-b",
			Args:  []string{"-c", "1b"},
			Stdin: bytes512(),
		},
		{
			Name:  "bytes-long-form",
			Args:  []string{"--bytes=5"},
			Stdin: []byte("abcdefghij"),
		},
		{
			Name:  "bytes-long-form-negative",
			Args:  []string{"--bytes=-3"},
			Stdin: []byte("abcdefghij"),
		},

		// R2.1: stdin via -
		{
			Name:  "bytes-stdin-dash",
			Args:  []string{"-c", "3", "-"},
			Stdin: []byte("hello"),
		},

		// Error: invalid byte count
		{
			Name:      "bytes-invalid",
			Args:      []string{"-c", "abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffHeaders(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghead")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	writeFixture(t, dir, "a.txt", "alpha\nbravo\ncharlie\n")
	writeFixture(t, dir, "b.txt", "delta\necho\nfoxtrot\n")

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		// R3.1: multi-file headers
		{
			Name:    "multi-file-headers",
			Args:    []string{"a.txt", "b.txt"},
			WorkDir: dir,
		},
		// R3.2: single file, no header by default
		{
			Name:    "single-file-no-header",
			Args:    []string{"a.txt"},
			WorkDir: dir,
		},
		// R3.3: -q suppresses headers on multiple files
		{
			Name:    "quiet-multi-file",
			Args:    []string{"-q", "a.txt", "b.txt"},
			WorkDir: dir,
		},
		// R3.3: --quiet long form
		{
			Name:    "quiet-long-multi-file",
			Args:    []string{"--quiet", "a.txt", "b.txt"},
			WorkDir: dir,
		},
		// R3.3: --silent long form
		{
			Name:    "silent-long-multi-file",
			Args:    []string{"--silent", "a.txt", "b.txt"},
			WorkDir: dir,
		},
		// R3.4: -v forces header on single file
		{
			Name:    "verbose-single-file",
			Args:    []string{"-v", "a.txt"},
			WorkDir: dir,
		},
		// R3.4: --verbose long form
		{
			Name:    "verbose-long-single-file",
			Args:    []string{"--verbose", "a.txt"},
			WorkDir: dir,
		},
		// R3.4: -v with multiple files
		{
			Name:    "verbose-multi-file",
			Args:    []string{"-v", "a.txt", "b.txt"},
			WorkDir: dir,
		},
		// R3.3+R3.4: last -q/-v wins
		{
			Name:    "quiet-then-verbose",
			Args:    []string{"-q", "-v", "a.txt"},
			WorkDir: dir,
		},
		{
			Name:    "verbose-then-quiet",
			Args:    []string{"-v", "-q", "a.txt", "b.txt"},
			WorkDir: dir,
		},
		// R3.1: multi-file with -n
		{
			Name:    "multi-file-with-n",
			Args:    []string{"-n", "1", "a.txt", "b.txt"},
			WorkDir: dir,
		},
		// R3.1: multi-file with -c
		{
			Name:    "multi-file-with-c",
			Args:    []string{"-c", "3", "a.txt", "b.txt"},
			WorkDir: dir,
		},
		// R3.5: error on non-existent file continues processing
		{
			Name:      "nonexistent-with-valid",
			Args:      []string{"nonexistent.txt", "a.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R3.5+R4.2: valid file then nonexistent
		{
			Name:      "valid-then-nonexistent",
			Args:      []string{"a.txt", "nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R3.5+R4.2: nonexistent between valid files
		{
			Name:      "nonexistent-between-valid",
			Args:      []string{"a.txt", "nonexistent.txt", "b.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R4.2: single nonexistent file
		{
			Name:      "nonexistent-only",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func bytes512() []byte {
	b := make([]byte, 600)
	for i := range b {
		b[i] = byte('a' + i%26)
	}
	return b
}

func seq(start, end int) []byte {
	var b []byte
	for i := start; i <= end; i++ {
		b = fmt.Appendf(b, "%d\n", i)
	}
	return b
}
