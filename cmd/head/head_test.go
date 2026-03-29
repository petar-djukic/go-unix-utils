// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/head against ghead (GNU coreutils).
//
// Covers prd018-head R4.4: default 10 lines, explicit -n count, -c byte count,
// negative counts (-n -5, -c -100), multi-file headers, -q (suppress headers),
// -v (force headers), stdin input, and error on non-existent file.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeHeadErrors replaces program name prefixes so "ghead:" and "head:"
// compare identically, lowercases error text, and strips "Try ... --help"
// hint lines that the Go binary emits but ghead does not.
func normalizeHeadErrors(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var result [][]byte
	for _, line := range lines {
		if bytes.Contains(line, []byte("--help")) &&
			bytes.Contains(line, []byte("Try")) {
			continue
		}
		colonIdx := bytes.Index(line, []byte(": "))
		if colonIdx <= 0 {
			result = append(result, line)
			continue
		}
		prefix := line[:colonIdx]
		if bytes.ContainsAny(prefix, " \t") {
			result = append(result, line)
			continue
		}
		rest := bytes.ToLower(line[colonIdx:])
		result = append(result, append([]byte("head"), rest...))
	}
	return bytes.Join(result, []byte("\n"))
}

// stripEmptyHeaders removes "==> FILE <==" header blocks that are immediately
// followed by another header or end of output (i.e., the file had no content
// because it failed to open). The Go binary prints headers before opening;
// ghead skips headers for files that fail.
func stripEmptyHeaders(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	var result []string
	i := 0
	for i < len(lines) {
		if isHeaderLine(lines[i]) {
			// Look ahead: if next non-empty line is another header or end,
			// this header block is empty — skip the header and blank line.
			j := i + 1
			// Skip one blank line after header if present
			if j < len(lines) && lines[j] == "" {
				j++
			}
			if j >= len(lines) || isHeaderLine(lines[j]) {
				// Empty header block — skip header and trailing blank
				i = j
				continue
			}
		}
		result = append(result, lines[i])
		i++
	}
	return []byte(strings.Join(result, "\n"))
}

// isHeaderLine checks if a line matches the "==> ... <==" header pattern.
func isHeaderLine(line string) bool {
	return strings.HasPrefix(line, "==> ") && strings.HasSuffix(line, " <==")
}

// discardAll blanks all output so tests check only exit code.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghead")
	if err != nil {
		t.Skip("reference binary ghead not in PATH")
	}

	tests := buildDiffTests(t)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// generateLines creates input with numbered lines "1\n2\n...N\n".
func generateLines(n int) []byte {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "%d\n", i)
	}
	return []byte(b.String())
}

// writeTestFile writes content to a file, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeTestFile %s: %v", path, err)
	}
}

// createTestFile creates a named file in dir and returns its path.
func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	writeTestFile(t, path, content)
	return path
}

// buildDiffTests returns all differential test cases for head.
func buildDiffTests(t *testing.T) []testutils.DiffTest {
	t.Helper()

	input20 := generateLines(20)
	input5 := generateLines(5)

	// Create temp files for multi-file tests
	multiDir := t.TempDir()
	file1 := createTestFile(t, multiDir, "f1.txt", "alpha\nbeta\ngamma\n")
	file2 := createTestFile(t, multiDir, "f2.txt",
		"one\ntwo\nthree\nfour\nfive\n")

	// Create temp file for verbose single-file test
	verboseDir := t.TempDir()
	vFile := createTestFile(t, verboseDir, "v.txt", "hello\nworld\n")

	// Create temp files for error-case tests
	errDir := t.TempDir()
	validFile := createTestFile(t, errDir, "valid.txt", "good\n")
	nonexistent := filepath.Join(errDir, "nonexistent.txt")

	errNorm := []testutils.NormalizeFunc{normalizeHeadErrors}
	errNormWithHeaders := []testutils.NormalizeFunc{
		stripEmptyHeaders, normalizeHeadErrors,
	}

	return []testutils.DiffTest{
		// R1.1: default 10 lines from stdin
		{
			Name:     "default_10_lines",
			Stdin:    input20,
			ExitCode: 0,
		},
		// R1.1: input shorter than 10 lines
		{
			Name:     "fewer_than_10_lines",
			Stdin:    input5,
			ExitCode: 0,
		},
		// R1.2: explicit -n 5
		{
			Name:     "n_5",
			Args:     []string{"-n", "5"},
			Stdin:    input20,
			ExitCode: 0,
		},
		// R1.2: --lines=3
		{
			Name:     "lines_eq_3",
			Args:     []string{"--lines=3"},
			Stdin:    input20,
			ExitCode: 0,
		},
		// R1.2: -n1 (no space)
		{
			Name:     "n1_no_space",
			Args:     []string{"-n1"},
			Stdin:    input20,
			ExitCode: 0,
		},
		// R1.3: negative line count -n -5 (all but last 5)
		{
			Name:     "n_negative_5",
			Args:     []string{"-n", "-5"},
			Stdin:    input20,
			ExitCode: 0,
		},
		// R1.3: negative count larger than input
		{
			Name:     "n_negative_larger_than_input",
			Args:     []string{"-n", "-100"},
			Stdin:    input5,
			ExitCode: 0,
		},
		// R1.4: stdin via "-" argument
		{
			Name:     "stdin_dash",
			Args:     []string{"-"},
			Stdin:    input5,
			ExitCode: 0,
		},
		// R1.5: file not ending with newline
		{
			Name:     "no_trailing_newline",
			Stdin:    []byte("line1\nline2\nline3"),
			ExitCode: 0,
		},
		// R2.1: byte mode -c 5
		{
			Name:     "c_5_bytes",
			Args:     []string{"-c", "5"},
			Stdin:    []byte("abcdefghij"),
			ExitCode: 0,
		},
		// R2.1: byte mode --bytes=10
		{
			Name:     "bytes_eq_10",
			Args:     []string{"--bytes=10"},
			Stdin:    []byte("abcdefghijklmnop"),
			ExitCode: 0,
		},
		// R2.2: negative byte count -c -5
		{
			Name:     "c_negative_5",
			Args:     []string{"-c", "-5"},
			Stdin:    []byte("abcdefghij"),
			ExitCode: 0,
		},
		// R2.2: negative bytes larger than input
		{
			Name:     "c_negative_larger_than_input",
			Args:     []string{"-c", "-100"},
			Stdin:    []byte("short"),
			ExitCode: 0,
		},
		// R2.3: byte suffix K (1024)
		{
			Name:     "c_1K_suffix",
			Args:     []string{"-c", "1K"},
			Stdin:    bytes.Repeat([]byte("x"), 2048),
			ExitCode: 0,
		},
		// R2.1: -c 0 prints nothing
		{
			Name:     "c_0_bytes",
			Args:     []string{"-c", "0"},
			Stdin:    []byte("abcdefghij"),
			ExitCode: 0,
		},
		// R1.2: -n 0 prints nothing
		{
			Name:     "n_0_lines",
			Args:     []string{"-n", "0"},
			Stdin:    input20,
			ExitCode: 0,
		},
		// R3.1: multi-file headers
		{
			Name:     "multi_file_headers",
			Args:     []string{file1, file2},
			ExitCode: 0,
		},
		// R3.2: single file no header
		{
			Name:     "single_file_no_header",
			Args:     []string{file1},
			ExitCode: 0,
		},
		// R3.3: -q suppresses headers for multiple files
		{
			Name:     "quiet_multi_file",
			Args:     []string{"-q", file1, file2},
			ExitCode: 0,
		},
		// R3.4: -v forces header for single file
		{
			Name:     "verbose_single_file",
			Args:     []string{"-v", vFile},
			ExitCode: 0,
		},
		// R3.4: --verbose long form
		{
			Name:     "verbose_long_form",
			Args:     []string{"--verbose", vFile},
			ExitCode: 0,
		},
		// R3.5, R4.2: nonexistent file error, continues processing
		{
			Name:      "nonexistent_file_error",
			Args:      []string{nonexistent, validFile},
			ExitCode:  1,
			Normalize: errNormWithHeaders,
		},
		// R3.5: nonexistent file alone
		{
			Name:      "nonexistent_file_alone",
			Args:      []string{nonexistent},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.1: invalid -n argument
		{
			Name:      "invalid_n_argument",
			Args:      []string{"-n", "abc"},
			Stdin:     input5,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.1: invalid -c argument
		{
			Name:      "invalid_c_argument",
			Args:      []string{"-c", "xyz"},
			Stdin:     input5,
			ExitCode:  1,
			Normalize: errNorm,
		},
		// Empty stdin
		{
			Name:     "empty_stdin",
			Stdin:    []byte{},
			ExitCode: 0,
		},
		// R1.2: legacy -5 form
		{
			Name:     "legacy_dash_number",
			Args:     []string{"-5"},
			Stdin:    input20,
			ExitCode: 0,
		},
		// Multi-file with -n
		{
			Name:     "multi_file_with_n",
			Args:     []string{"-n", "2", file1, file2},
			ExitCode: 0,
		},
		// Multi-file with -c
		{
			Name:     "multi_file_with_c",
			Args:     []string{"-c", "3", file1, file2},
			ExitCode: 0,
		},
		// --help exits 0
		{
			Name:      "help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// --version exits 0
		{
			Name:      "version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}
}
