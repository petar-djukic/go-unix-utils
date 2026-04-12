// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/tail.
// Tests cover srd055-tail R4.1, R4.2, R4.3, R4.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgRe matches the program name/path prefix before a colon at line start.
var stderrProgRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// tryLineRe matches the "Try '...' for more information." line in stderr.
var tryLineRe = regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)

// stderrNormalizer normalizes program name, path, and case differences.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = tryLineRe.ReplaceAll(b, []byte("Try 'PROG --help' for more information.\n"))
	b = bytes.ToLower(b)
	return b
}

// versionNormalizer replaces version output with a constant so both binaries match.
// R4.4: --version outputs differ between GNU and Go implementations; we verify
// both exit 0 and produce output containing the program name.
func versionNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	lower := bytes.ToLower(b)
	if bytes.Contains(lower, []byte("tail")) {
		return []byte("tail version\n")
	}
	return b
}

// helpNormalizer replaces help output with a constant so both binaries match.
// R4.4: --help outputs differ in detail; we verify both exit 0 and produce
// usage text starting with "Usage:".
func helpNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	if bytes.HasPrefix(b, []byte("Usage:")) {
		return []byte("usage output\n")
	}
	return b
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtail")
	if err != nil {
		t.Skipf("reference binary gtail not in PATH: %v", err)
	}

	// Create fixture files.
	fixtureDir := t.TempDir()
	writeFixture(t, fixtureDir, "file1.txt", "1\n2\n")
	writeFixture(t, fixtureDir, "file2.txt", "3\n4\n")
	writeFixture(t, fixtureDir, "data.txt", "data\n")
	writeFixture(t, fixtureDir, "three.txt", "1\n2\n3\n")
	writeFixture(t, fixtureDir, "twenty.txt", genLines(20))
	writeFixture(t, fixtureDir, "ten.txt", genLines(10))
	writeFixture(t, fixtureDir, "short.txt", "1\n2\n")
	writeFixture(t, fixtureDir, "bytes.txt", "abcdefgh")
	writeFixture(t, fixtureDir, "1k.txt", strings.Repeat("a", 2048))
	writeFixture(t, fixtureDir, "empty.txt", "")
	writeFixture(t, fixtureDir, "noeof.txt", "no trailing newline")

	file1 := filepath.Join(fixtureDir, "file1.txt")
	file2 := filepath.Join(fixtureDir, "file2.txt")
	dataFile := filepath.Join(fixtureDir, "data.txt")
	threeFile := filepath.Join(fixtureDir, "three.txt")
	twentyFile := filepath.Join(fixtureDir, "twenty.txt")
	tenFile := filepath.Join(fixtureDir, "ten.txt")
	shortFile := filepath.Join(fixtureDir, "short.txt")
	bytesFile := filepath.Join(fixtureDir, "bytes.txt")
	oneKFile := filepath.Join(fixtureDir, "1k.txt")
	emptyFile := filepath.Join(fixtureDir, "empty.txt")
	noeofFile := filepath.Join(fixtureDir, "noeof.txt")
	missingFile := filepath.Join(fixtureDir, "missing.txt")

	tests := []testutils.DiffTest{
		// R4.1: default 10 lines from stdin.
		{
			Name:  "default_10_lines",
			Stdin: []byte(genLines(20)),
		},
		// R4.1: default with fewer than 10 lines.
		{
			Name:  "default_fewer_lines",
			Stdin: []byte(genLines(3)),
		},
		// R4.3: explicit -n 5.
		{
			Name:  "n_5",
			Args:  []string{"-n", "5"},
			Stdin: []byte(genLines(20)),
		},
		// R4.3: -n from file.
		{
			Name: "n_3_file",
			Args: []string{"-n", "3", twentyFile},
		},
		// R4.3: -c byte count.
		{
			Name:  "c_5",
			Args:  []string{"-c", "5"},
			Stdin: []byte("abcdefghij"),
		},
		// R4.3: -c from file.
		{
			Name: "c_4_file",
			Args: []string{"-c", "4", bytesFile},
		},
		// R4.3: +N offset lines -- start from line 5.
		{
			Name:  "n_plus_5",
			Args:  []string{"-n", "+5"},
			Stdin: []byte(genLines(10)),
		},
		// R4.3: +N offset lines from file.
		{
			Name: "n_plus_3_file",
			Args: []string{"-n", "+3", tenFile},
		},
		// R4.3: +N offset bytes -- start from byte 5.
		{
			Name:  "c_plus_5",
			Args:  []string{"-c", "+5"},
			Stdin: []byte("abcdefghij"),
		},
		// R4.3: +N offset bytes from file.
		{
			Name: "c_plus_3_file",
			Args: []string{"-c", "+3", bytesFile},
		},
		// R4.3: multi-file headers.
		{
			Name: "multi_file_headers",
			Args: []string{file1, file2},
		},
		// R4.3: single file no header.
		{
			Name: "single_file_no_header",
			Args: []string{threeFile},
		},
		// R4.3: -q suppresses headers for multiple files.
		{
			Name: "quiet_multi_file",
			Args: []string{"-q", file1, file2},
		},
		// R4.3: --quiet long form.
		{
			Name: "quiet_long_multi_file",
			Args: []string{"--quiet", file1, file2},
		},
		// R4.3: --silent long form.
		{
			Name: "silent_multi_file",
			Args: []string{"--silent", file1, file2},
		},
		// R4.3: -v forces header for single file.
		{
			Name: "verbose_single_file",
			Args: []string{"-v", dataFile},
		},
		// R4.3: --verbose long form.
		{
			Name: "verbose_long_single_file",
			Args: []string{"--verbose", dataFile},
		},
		// R4.3: -v with multiple files.
		{
			Name: "verbose_multi_file",
			Args: []string{"-v", file1, file2},
		},
		// R4.3: last flag wins: -v then -q suppresses.
		{
			Name: "v_then_q",
			Args: []string{"-v", "-q", file1, file2},
		},
		// R4.3: last flag wins: -q then -v shows headers.
		{
			Name: "q_then_v",
			Args: []string{"-q", "-v", file1},
		},
		// R4.3: stdin input via "-".
		{
			Name:  "stdin_dash",
			Args:  []string{"-n", "2", "-"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R4.3: non-existent file exits 1.
		{
			Name:      "missing_file",
			Args:      []string{missingFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.2: non-existent file continues with next.
		{
			Name:      "missing_file_continues",
			Args:      []string{missingFile, dataFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.3: suffix multiplier with -c.
		{
			Name: "c_1K_suffix",
			Args: []string{"-c", "1K", oneKFile},
		},
		// R4.1: -n 0 outputs nothing.
		{
			Name:  "n_zero",
			Args:  []string{"-n", "0"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R4.1: -c 0 outputs nothing.
		{
			Name:  "c_zero",
			Args:  []string{"-c", "0"},
			Stdin: []byte("abcdefgh"),
		},
		// R4.1: empty stdin.
		{
			Name:  "empty_stdin",
			Stdin: []byte(""),
		},
		// R4.1: empty file.
		{
			Name: "empty_file",
			Args: []string{emptyFile},
		},
		// R4.3: file without trailing newline.
		{
			Name: "file_no_trailing_newline",
			Args: []string{noeofFile},
		},
		// R4.3: file shorter than requested line count.
		{
			Name: "file_shorter_than_n",
			Args: []string{"-n", "100", shortFile},
		},
		// R4.3: file shorter than requested byte count.
		{
			Name: "file_shorter_than_c",
			Args: []string{"-c", "1000", bytesFile},
		},
		// R4.3: --lines= long form.
		{
			Name:  "lines_long_form",
			Args:  []string{"--lines=3"},
			Stdin: []byte(genLines(10)),
		},
		// R4.3: --bytes= long form.
		{
			Name:  "bytes_long_form",
			Args:  []string{"--bytes=4"},
			Stdin: []byte("abcdefgh"),
		},
		// R4.3: -c and -n mutually exclusive, last wins.
		{
			Name:  "c_overrides_n",
			Args:  []string{"-n", "1", "-c", "3"},
			Stdin: []byte("abcdefghij\n"),
		},
		// R4.3: -n overrides -c.
		{
			Name:  "n_overrides_c",
			Args:  []string{"-c", "3", "-n", "1"},
			Stdin: []byte("abcdefghij\n"),
		},
		// R4.3: multi-file with empty file.
		{
			Name: "multi_file_with_empty",
			Args: []string{file1, emptyFile, file2},
		},
		// R4.3: verbose stdin header shows "standard input".
		{
			Name:  "verbose_stdin_header",
			Args:  []string{"-v", "-n", "1", "-"},
			Stdin: []byte("hello\n"),
		},
		// R4.4: --version prints version info.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{versionNormalizer},
		},
		// R4.4: --help prints usage.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{helpNormalizer},
		},
		// R4.4: unrecognized short option.
		{
			Name:      "bad_short_option",
			Args:      []string{"-Z"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.4: unrecognized long option.
		{
			Name:      "bad_long_option",
			Args:      []string{"--nonexistent"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.4: invalid -n argument.
		{
			Name:      "invalid_n_nonnumeric",
			Args:      []string{"-n", "abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.4: invalid -c argument.
		{
			Name:      "invalid_c_nonnumeric",
			Args:      []string{"-c", "xyz"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.2: missing file between two valid files.
		{
			Name:      "missing_between_valid",
			Args:      []string{file1, missingFile, file2},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.3: +1 prints all lines (start from line 1).
		{
			Name:  "n_plus_1_all",
			Args:  []string{"-n", "+1"},
			Stdin: []byte(genLines(5)),
		},
		// R4.3: +1 bytes prints all bytes.
		{
			Name:  "c_plus_1_all",
			Args:  []string{"-c", "+1"},
			Stdin: []byte("abcde"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeFixture creates a file in dir with the given content.
func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

// genLines returns a string with numbered lines "1\n2\n...n\n".
func genLines(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString(itoa(i))
		b.WriteByte('\n')
	}
	return b.String()
}

// itoa converts an int to string without importing strconv in the test file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
