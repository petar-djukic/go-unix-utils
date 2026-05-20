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
	refBin, err := exec.LookPath("gtail")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		// R1.1: default last 10 lines
		{
			Name:  "default-10-lines",
			Stdin: seq(1, 20),
		},
		// R1.1: fewer than 10 lines outputs all
		{
			Name:  "default-fewer-than-10",
			Stdin: seq(1, 5),
		},
		// R1.2: explicit -n count
		{
			Name:  "explicit-n-5",
			Args:  []string{"-n", "5"},
			Stdin: seq(1, 20),
		},
		// R1.2: -n with attached value
		{
			Name:  "n-attached-value",
			Args:  []string{"-n5"},
			Stdin: seq(1, 20),
		},
		// R1.2: --lines= long form
		{
			Name:  "lines-equals",
			Args:  []string{"--lines=5"},
			Stdin: seq(1, 20),
		},
		// R1.2: --lines separate argument
		{
			Name:  "lines-separate",
			Args:  []string{"--lines", "5"},
			Stdin: seq(1, 20),
		},
		// R1.3: +N starts from line N
		{
			Name:  "plus-offset-5",
			Args:  []string{"-n", "+5"},
			Stdin: seq(1, 20),
		},
		// R1.3: +1 outputs everything
		{
			Name:  "plus-offset-1",
			Args:  []string{"-n", "+1"},
			Stdin: seq(1, 10),
		},
		// R1.3: +N exceeds input
		{
			Name:  "plus-offset-exceeds",
			Args:  []string{"-n", "+100"},
			Stdin: seq(1, 10),
		},
		// R1.4: stdin with no arguments
		{
			Name:  "stdin-no-args",
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		// R1.4: stdin via explicit -
		{
			Name:  "stdin-dash",
			Args:  []string{"-"},
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		// Edge: no trailing newline
		{
			Name:  "no-trailing-newline",
			Args:  []string{"-n", "3"},
			Stdin: []byte("a\nb\nc"),
		},
		// Edge: count exceeds input
		{
			Name:  "count-exceeds-input",
			Args:  []string{"-n", "100"},
			Stdin: []byte("one\ntwo\n"),
		},
		// Edge: -n 0 outputs nothing
		{
			Name:  "n-zero",
			Args:  []string{"-n", "0"},
			Stdin: seq(1, 10),
		},
		// Edge: empty input
		{
			Name:  "empty-input",
			Stdin: []byte{},
		},
		// Edge: single line
		{
			Name:  "single-line",
			Stdin: []byte("only\n"),
		},
		// Error: invalid number
		{
			Name:      "invalid-number",
			Args:      []string{"-n", "abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.1: -c NUM last N bytes
		{
			Name:  "bytes-last-5",
			Args:  []string{"-c", "5"},
			Stdin: []byte("abcdefghij"),
		},
		// R2.1: -c with attached value
		{
			Name:  "bytes-attached-value",
			Args:  []string{"-c5"},
			Stdin: []byte("abcdefghij"),
		},
		// R2.1: --bytes= long form
		{
			Name:  "bytes-equals",
			Args:  []string{"--bytes=5"},
			Stdin: []byte("abcdefghij"),
		},
		// R2.1: --bytes separate argument
		{
			Name:  "bytes-separate",
			Args:  []string{"--bytes", "5"},
			Stdin: []byte("abcdefghij"),
		},
		// R2.1: -c exceeds input length
		{
			Name:  "bytes-exceeds-input",
			Args:  []string{"-c", "100"},
			Stdin: []byte("short"),
		},
		// R2.1: -c 0 outputs nothing
		{
			Name:  "bytes-zero",
			Args:  []string{"-c", "0"},
			Stdin: []byte("abcdefghij"),
		},
		// R2.1: -c on empty input
		{
			Name:  "bytes-empty-input",
			Args:  []string{"-c", "5"},
			Stdin: []byte{},
		},
		// R2.1: last option wins (-n then -c)
		{
			Name:  "bytes-last-wins-c",
			Args:  []string{"-n", "2", "-c", "3"},
			Stdin: []byte("abcdefghij\nklmnopqrst\n"),
		},
		// R2.1: last option wins (-c then -n)
		{
			Name:  "bytes-last-wins-n",
			Args:  []string{"-c", "3", "-n", "1"},
			Stdin: []byte("abcdefghij\nklmnopqrst\n"),
		},
		// R2.2: -c +N from byte N
		{
			Name:  "bytes-plus-offset",
			Args:  []string{"-c", "+5"},
			Stdin: []byte("abcdefghij"),
		},
		// R2.2: -c +1 outputs everything
		{
			Name:  "bytes-plus-1",
			Args:  []string{"-c", "+1"},
			Stdin: []byte("abcdefghij"),
		},
		// R2.2: -c +N exceeds input
		{
			Name:  "bytes-plus-exceeds",
			Args:  []string{"-c", "+100"},
			Stdin: []byte("short"),
		},
		// R2.3: multiplier suffix K
		{
			Name:  "bytes-suffix-K",
			Args:  []string{"-c", "1K"},
			Stdin: seq(1, 200),
		},
		// R2.3: multiplier suffix b (512-byte blocks)
		{
			Name:  "bytes-suffix-b",
			Args:  []string{"-c", "1b"},
			Stdin: seq(1, 200),
		},
		// Error: invalid byte count
		{
			Name:      "invalid-byte-count",
			Args:      []string{"-c", "abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtail")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	dir := t.TempDir()
	writeFixture(t, dir, "input.txt", string(seq(1, 20)))
	writeFixture(t, dir, "a.txt", string(seq(1, 5)))
	writeFixture(t, dir, "b.txt", string(seq(6, 10)))

	tests := []testutils.DiffTest{
		// R1.1: default from file
		{
			Name:    "file-default",
			Args:    []string{"input.txt"},
			WorkDir: dir,
		},
		// R1.2: explicit -n from file
		{
			Name:    "file-n-5",
			Args:    []string{"-n", "5", "input.txt"},
			WorkDir: dir,
		},
		// R1.3: +N from file
		{
			Name:    "file-plus-offset",
			Args:    []string{"-n", "+5", "input.txt"},
			WorkDir: dir,
		},
		// R2.1: byte mode from file
		{
			Name:    "file-bytes-5",
			Args:    []string{"-c", "5", "input.txt"},
			WorkDir: dir,
		},
		// R2.2: byte +offset from file
		{
			Name:    "file-bytes-plus-offset",
			Args:    []string{"-c", "+10", "input.txt"},
			WorkDir: dir,
		},
		// R4.2, R4.4: nonexistent file
		{
			Name:      "nonexistent-file",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R3.1: multi-file headers
		{
			Name:    "multi-file-headers",
			Args:    []string{"a.txt", "b.txt"},
			WorkDir: dir,
		},
		// R3.1: multi-file with -n
		{
			Name:    "multi-file-n",
			Args:    []string{"-n", "3", "a.txt", "b.txt"},
			WorkDir: dir,
		},
		// R3.2: single file no header
		{
			Name:    "single-file-no-header",
			Args:    []string{"a.txt"},
			WorkDir: dir,
		},
		// R3.3: -q suppresses headers for multiple files
		{
			Name:    "quiet-multi-file",
			Args:    []string{"-q", "a.txt", "b.txt"},
			WorkDir: dir,
		},
		// R3.3: --quiet suppresses headers
		{
			Name:    "quiet-long-multi-file",
			Args:    []string{"--quiet", "a.txt", "b.txt"},
			WorkDir: dir,
		},
		// R3.3: --silent suppresses headers
		{
			Name:    "silent-multi-file",
			Args:    []string{"--silent", "a.txt", "b.txt"},
			WorkDir: dir,
		},
		// R3.4: -v shows header for single file
		{
			Name:    "verbose-single-file",
			Args:    []string{"-v", "a.txt"},
			WorkDir: dir,
		},
		// R3.4: --verbose shows header for single file
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
		// R3.3+R4.2: -q with nonexistent file among valid files
		{
			Name:      "quiet-with-error",
			Args:      []string{"-q", "a.txt", "nonexistent.txt", "b.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R4.2+R4.4: nonexistent file between valid files still outputs valid files with headers
		{
			Name:      "error-mixed-valid-invalid",
			Args:      []string{"a.txt", "nonexistent.txt", "b.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R4.2+R4.4: nonexistent first, valid file still processed
		{
			Name:      "error-first-nonexistent",
			Args:      []string{"nonexistent.txt", "a.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R4.2+R4.4: valid file then nonexistent, valid output still appears
		{
			Name:      "error-last-nonexistent",
			Args:      []string{"a.txt", "nonexistent.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R4.2: multiple nonexistent files
		{
			Name:      "error-all-nonexistent",
			Args:      []string{"no1.txt", "no2.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R4.2+R3.4: -v with error still shows headers for valid files
		{
			Name:      "verbose-with-error",
			Args:      []string{"-v", "a.txt", "nonexistent.txt"},
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

func seq(start, end int) []byte {
	var b []byte
	for i := start; i <= end; i++ {
		b = fmt.Appendf(b, "%d\n", i)
	}
	return b
}
