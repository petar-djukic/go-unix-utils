// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/head.
// Tests cover srd018-head R1.1-R1.5, R2.1-R2.3, R3.1-R3.4, R4.1-R4.4.
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

// stderrNormalizer normalizes program name and error message case differences.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = bytes.ToLower(b)
	return b
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghead")
	if err != nil {
		t.Skipf("reference binary ghead not in PATH: %v", err)
	}

	// Create fixture files for multi-file and header tests.
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
	writeFixture(t, fixtureDir, "binary.bin", "\x00\x01\x02\xff\xfe\x80hello\x00world\n")
	writeFixture(t, fixtureDir, "noeof.txt", "no trailing newline")

	// Create an unreadable file for permission-denied testing.
	unreadablePath := filepath.Join(fixtureDir, "noperm.txt")
	writeFixture(t, fixtureDir, "noperm.txt", "secret\n")
	os.Chmod(unreadablePath, 0o000) //nolint:errcheck // best-effort for test setup

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
	binaryFile := filepath.Join(fixtureDir, "binary.bin")
	noeofFile := filepath.Join(fixtureDir, "noeof.txt")
	missingFile := filepath.Join(fixtureDir, "missing.txt")

	tests := []testutils.DiffTest{
		// R1.1: default 10 lines from stdin.
		{
			Name:  "default_10_lines",
			Stdin: []byte(genLines(12)),
		},
		// R1.2: explicit -n 5.
		{
			Name:  "n_5",
			Args:  []string{"-n", "5"},
			Stdin: []byte(genLines(10)),
		},
		// R1.3: negative line count -n -5 prints all but last 5.
		{
			Name:  "n_negative_5",
			Args:  []string{"-n", "-5"},
			Stdin: []byte(genLines(10)),
		},
		// R1.3: negative line count from file.
		{
			Name: "n_negative_3_file",
			Args: []string{"-n", "-3", twentyFile},
		},
		// R1.4: stdin via "-".
		{
			Name:  "stdin_dash",
			Args:  []string{"-n", "2", "-"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R1.5: no trailing newline still counts as a line.
		{
			Name:  "no_trailing_newline",
			Args:  []string{"-n", "2"},
			Stdin: []byte("a\nb"),
		},
		// R1.2: fewer lines than requested.
		{
			Name:  "fewer_lines_than_n",
			Args:  []string{"-n", "100"},
			Stdin: []byte("a\nb\n"),
		},
		// R1.1: empty stdin.
		{
			Name:  "empty_stdin",
			Stdin: []byte(""),
		},
		// R2.1: byte count -c 5.
		{
			Name:  "c_5",
			Args:  []string{"-c", "5"},
			Stdin: []byte("abcdefghij"),
		},
		// R2.2: negative byte count -c -3 prints all but last 3 bytes.
		{
			Name:  "c_negative_3",
			Args:  []string{"-c", "-3"},
			Stdin: []byte("abcdefgh"),
		},
		// R2.2: negative byte count where input is shorter than count.
		{
			Name:  "c_negative_100_short",
			Args:  []string{"-c", "-100"},
			Stdin: []byte("short\n"),
		},
		// R2.2: negative byte count from file.
		{
			Name: "c_negative_5_file",
			Args: []string{"-c", "-5", bytesFile},
		},
		// R2.3: byte count with K suffix.
		{
			Name: "c_1K",
			Args: []string{"-c", "1K", oneKFile},
		},
		// R3.1: multi-file headers.
		{
			Name: "multi_file_headers",
			Args: []string{file1, file2},
		},
		// R3.2: single file, no header.
		{
			Name: "single_file_no_header",
			Args: []string{threeFile},
		},
		// R3.3: -q suppresses headers for multiple files.
		{
			Name: "quiet_multi_file",
			Args: []string{"-q", file1, file2},
		},
		// R3.3: --quiet long form.
		{
			Name: "quiet_long_multi_file",
			Args: []string{"--quiet", file1, file2},
		},
		// R3.3: --silent long form.
		{
			Name: "silent_multi_file",
			Args: []string{"--silent", file1, file2},
		},
		// R3.4: -v forces header for single file.
		{
			Name: "verbose_single_file",
			Args: []string{"-v", dataFile},
		},
		// R3.4: --verbose long form.
		{
			Name: "verbose_long_single_file",
			Args: []string{"--verbose", dataFile},
		},
		// R3.4: -v with multiple files (headers shown).
		{
			Name: "verbose_multi_file",
			Args: []string{"-v", file1, file2},
		},
		// R3.3/R3.4: last flag wins: -v then -q suppresses.
		{
			Name: "v_then_q",
			Args: []string{"-v", "-q", file1, file2},
		},
		// R3.3/R3.4: last flag wins: -q then -v shows headers.
		{
			Name: "q_then_v",
			Args: []string{"-q", "-v", file1},
		},
		// R3.5, R4.2: non-existent file, continues with next.
		{
			Name:      "missing_file_continues",
			Args:      []string{missingFile, dataFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R1.3: negative count larger than input.
		{
			Name:  "n_negative_exceeds_input",
			Args:  []string{"-n", "-100"},
			Stdin: []byte("a\nb\n"),
		},
		// R2.1: -c and -n mutually exclusive, last wins.
		{
			Name:  "c_overrides_n",
			Args:  []string{"-n", "1", "-c", "3"},
			Stdin: []byte("abcdefghij\n"),
		},
		// R2.1: -n overrides -c.
		{
			Name:  "n_overrides_c",
			Args:  []string{"-c", "3", "-n", "1"},
			Stdin: []byte("abcdefghij\n"),
		},
		// R1.2: --lines= long form.
		{
			Name:  "lines_long_form",
			Args:  []string{"--lines=3"},
			Stdin: []byte(genLines(10)),
		},
		// R2.1: --bytes= long form.
		{
			Name:  "bytes_long_form",
			Args:  []string{"--bytes=4"},
			Stdin: []byte("abcdefgh"),
		},
		// Multi-file with negative count.
		{
			Name: "n_negative_multi_file",
			Args: []string{"-n", "-1", tenFile, shortFile},
		},
		// Quiet with negative count.
		{
			Name: "quiet_negative_n",
			Args: []string{"-q", "-n", "-2", tenFile, shortFile},
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
		// R4.1: -n 0 with file.
		{
			Name: "n_zero_file",
			Args: []string{"-n", "0", threeFile},
		},
		// R4.1: -c 0 with file.
		{
			Name: "c_zero_file",
			Args: []string{"-c", "0", bytesFile},
		},
		// R4.2: binary file, byte-identical output.
		{
			Name: "binary_file",
			Args: []string{binaryFile},
		},
		// R4.2: binary file with -c byte mode.
		{
			Name: "binary_file_c5",
			Args: []string{"-c", "5", binaryFile},
		},
		// R4.3: empty file produces no output.
		{
			Name: "empty_file",
			Args: []string{emptyFile},
		},
		// R4.3: empty file with -c.
		{
			Name: "empty_file_c10",
			Args: []string{"-c", "10", emptyFile},
		},
		// R4.3: empty file with verbose header.
		{
			Name: "empty_file_verbose",
			Args: []string{"-v", emptyFile},
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
		// R4.2: file without trailing newline.
		{
			Name: "file_no_trailing_newline",
			Args: []string{noeofFile},
		},
		// R4.2: file without trailing newline in byte mode.
		{
			Name: "file_no_trailing_newline_c5",
			Args: []string{"-c", "5", noeofFile},
		},
		// R3.5, R4.2: permission denied continues with next file.
		{
			Name:      "permission_denied_continues",
			Args:      []string{unreadablePath, dataFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R3.5: missing file between two valid files.
		{
			Name:      "missing_between_valid",
			Args:      []string{file1, missingFile, file2},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R3.1: verbose stdin header shows "standard input".
		{
			Name:  "verbose_stdin_header",
			Args:  []string{"-v", "-n", "1", "-"},
			Stdin: []byte("hello\n"),
		},
		// R4.3: multi-file with empty file.
		{
			Name: "multi_file_with_empty",
			Args: []string{file1, emptyFile, file2},
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
		b.WriteString(strings.Repeat("", 0))
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
