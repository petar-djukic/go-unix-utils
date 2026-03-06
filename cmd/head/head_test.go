// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinName is the Homebrew GNU reference binary for head.
const refBinName = "ghead"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	// Create fixture files for file-based tests.
	dir := t.TempDir()
	writeFixture(t, dir, "a.txt", "1\n2\n")
	writeFixture(t, dir, "b.txt", "3\n4\n")
	writeFixture(t, dir, "single.txt", "1\n2\n3\n")
	writeFixture(t, dir, "data.txt", "data\n")
	writeFixture(t, dir, "qa.txt", "1\n")
	writeFixture(t, dir, "qb.txt", "2\n")

	// Generate 2048 bytes of 'a' for suffix test.
	writeFixture(t, dir, "big.txt", strings.Repeat("a", 2048))

	stdinLines12 := []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n")
	stdinLines10 := []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n")

	errNorm := normalizeErrPrefix()

	tests := []testutils.DiffTest{
		// R1.1: Default 10 lines from stdin.
		{
			Name:  "default_10_lines",
			Stdin: stdinLines12,
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: Explicit -n count.
		{
			Name:  "n_5",
			Args:  []string{"-n", "5"},
			Stdin: stdinLines10,
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: Negative line count.
		{
			Name:  "n_negative_5",
			Args:  []string{"-n", "-5"},
			Stdin: stdinLines10,
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: Byte count.
		{
			Name:  "c_5",
			Args:  []string{"-c", "5"},
			Stdin: []byte("abcdefghij"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: Negative byte count (short input, all within last N).
		{
			Name:  "c_negative_100",
			Args:  []string{"-c", "-100"},
			Stdin: []byte("short\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: Negative byte count with sufficient input.
		{
			Name:  "c_negative_3",
			Args:  []string{"-c", "-3"},
			Stdin: []byte("abcdefgh"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: Byte count with K suffix.
		{
			Name:    "c_1K_suffix",
			Args:    []string{"-c", "1K", "big.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		// R1.4: Stdin via dash.
		{
			Name:  "stdin_dash",
			Args:  []string{"-n", "2", "-"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: Multi-file headers.
		{
			Name:    "multi_file_headers",
			Args:    []string{"a.txt", "b.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		// R3.2: Single file, no header.
		{
			Name:    "single_file_no_header",
			Args:    []string{"single.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		// R3.3: Quiet mode suppresses headers.
		{
			Name:    "quiet_multi_file",
			Args:    []string{"-q", "qa.txt", "qb.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		// R3.4: Verbose mode forces headers for single file.
		{
			Name:    "verbose_single_file",
			Args:    []string{"-v", "data.txt"},
			WorkDir: dir,
			Env:     []string{"LC_ALL=C"},
		},
		// R3.5, R4.2: Missing file with existing file.
		{
			Name:      "missing_file_error",
			Args:      []string{"nosuchfile.txt", "data.txt"},
			WorkDir:   dir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R1.2: Fewer lines than requested.
		{
			Name:  "fewer_lines_than_n",
			Args:  []string{"-n", "100"},
			Stdin: []byte("a\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Empty stdin.
		{
			Name:  "empty_stdin",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: No trailing newline.
		{
			Name:  "no_trailing_newline",
			Args:  []string{"-n", "2"},
			Stdin: []byte("a\nb"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

// normalizeErrPrefix returns a NormalizeFunc that normalizes binary name
// prefixes in stderr (e.g. "ghead:" or "/tmp/.../head:") to "head:".
func normalizeErrPrefix() testutils.NormalizeFunc {
	re := regexp.MustCompile(`(?m)^[^\s:]*head[^\s:]*:`)
	return func(b []byte) []byte {
		return re.ReplaceAll(b, []byte("head:"))
	}
}
