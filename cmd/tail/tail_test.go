// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tail against gtail (GNU coreutils).
//
// Covers prd055-tail R1.1, R1.2, R1.3, R1.4: default 10 lines, explicit -n
// count, --lines= form, +N offset, stdin input, stdin via "-" argument.
// Covers prd055-tail R2.1, R2.2, R2.3: byte-count mode, byte offset, suffixes.
// Covers prd055-tail R3.1, R3.2, R3.3, R3.4: multi-file headers, quiet, verbose.
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

// normalizeTailErrors replaces program name prefixes so "gtail:" and "tail:"
// compare identically, lowercases error text, and strips "Try ... --help"
// hint lines.
func normalizeTailErrors(data []byte) []byte {
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
		result = append(result, append([]byte("tail"), rest...))
	}
	return bytes.Join(result, []byte("\n"))
}

// stripEmptyHeaders removes "==> FILE <==" header blocks that are immediately
// followed by another header or end of output.
func stripEmptyHeaders(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	var result []string
	i := 0
	for i < len(lines) {
		if isHeaderLine(lines[i]) {
			j := i + 1
			if j < len(lines) && lines[j] == "" {
				j++
			}
			if j >= len(lines) || isHeaderLine(lines[j]) {
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

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtail")
	if err != nil {
		t.Skip("reference binary gtail not in PATH")
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

// createTestFile creates a named file in dir and returns its path.
func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("createTestFile %s: %v", path, err)
	}
	return path
}

// buildDiffTests returns all differential test cases for tail.
func buildDiffTests(t *testing.T) []testutils.DiffTest {
	t.Helper()

	input12 := generateLines(12)
	input20 := generateLines(20)
	input5 := generateLines(5)

	// Create temp files for multi-file and file-based tests.
	tmpDir := t.TempDir()
	file1 := createTestFile(t, tmpDir, "f1.txt", string(input12))
	file2 := createTestFile(t, tmpDir, "f2.txt", string(input12))

	errDir := t.TempDir()
	validFile := createTestFile(t, errDir, "valid.txt", "good\n")
	nonexistent := filepath.Join(errDir, "nonexistent.txt")

	errNorm := []testutils.NormalizeFunc{normalizeTailErrors}
	errNormWithHeaders := []testutils.NormalizeFunc{
		stripEmptyHeaders, normalizeTailErrors,
	}

	lineTests := buildLineTests(input5, input12, input20, file1, file2, validFile, nonexistent, errNorm, errNormWithHeaders)
	byteTests := buildByteTests(t)
	headerTests := buildHeaderTests(t)
	tests := append(lineTests, byteTests...)
	return append(tests, headerTests...)
}

// buildLineTests returns differential tests for R1.1-R1.4 (line mode).
func buildLineTests(input5, input12, input20 []byte, file1, file2, validFile, nonexistent string, errNorm, errNormWithHeaders []testutils.NormalizeFunc) []testutils.DiffTest {
	return []testutils.DiffTest{
		// R1.1: default 10 lines from stdin
		{
			Name:     "default_10_lines",
			Stdin:    input12,
			ExitCode: 0,
		},
		// R1.1: input shorter than 10 lines prints all
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
		// R1.2: -n 0 prints nothing
		{
			Name:     "n_0_lines",
			Args:     []string{"-n", "0"},
			Stdin:    input20,
			ExitCode: 0,
		},
		// R1.3: plus offset -n +5
		{
			Name:     "n_plus_5",
			Args:     []string{"-n", "+5"},
			Stdin:    input20,
			ExitCode: 0,
		},
		// R1.3: plus offset -n +1 prints entire file
		{
			Name:     "n_plus_1_all",
			Args:     []string{"-n", "+1"},
			Stdin:    input5,
			ExitCode: 0,
		},
		// R1.3: plus offset beyond input length
		{
			Name:     "n_plus_beyond_input",
			Args:     []string{"-n", "+100"},
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
		// R1.4: stdin with -n flag
		{
			Name:     "stdin_with_n",
			Args:     []string{"-n", "3"},
			Stdin:    input12,
			ExitCode: 0,
		},
		// R1.1: empty stdin
		{
			Name:     "empty_stdin",
			Stdin:    []byte{},
			ExitCode: 0,
		},
		// R1.1: input without trailing newline
		{
			Name:     "no_trailing_newline",
			Stdin:    []byte("line1\nline2\nline3"),
			ExitCode: 0,
		},
		// R1.2: legacy -5 form
		{
			Name:     "legacy_dash_number",
			Args:     []string{"-5"},
			Stdin:    input20,
			ExitCode: 0,
		},
		// R1.1, R1.4: multi-file with default lines
		{
			Name:     "multi_file_default",
			Args:     []string{file1, file2},
			ExitCode: 0,
		},
		// R1.2: multi-file with -n
		{
			Name:     "multi_file_with_n",
			Args:     []string{"-n", "2", file1, file2},
			ExitCode: 0,
		},
		// R4.2, R4.4: nonexistent file, continues processing
		{
			Name:      "nonexistent_file_with_valid",
			Args:      []string{nonexistent, validFile},
			ExitCode:  1,
			Normalize: errNormWithHeaders,
		},
		// R4.2: nonexistent file alone
		{
			Name:      "nonexistent_file_alone",
			Args:      []string{nonexistent},
			ExitCode:  1,
			Normalize: errNorm,
		},
	}
}

// buildHeaderTests returns differential tests for R3.1-R3.4 (multi-file headers).
func buildHeaderTests(t *testing.T) []testutils.DiffTest {
	t.Helper()

	hDir := t.TempDir()
	hf1 := createTestFile(t, hDir, "a.txt", "alpha\nbeta\n")
	hf2 := createTestFile(t, hDir, "b.txt", "gamma\ndelta\n")
	hf3 := createTestFile(t, hDir, "c.txt", "epsilon\n")

	return []testutils.DiffTest{
		// R3.1: multiple files get headers
		{
			Name:     "R3.1_multi_file_headers",
			Args:     []string{hf1, hf2},
			ExitCode: 0,
		},
		// R3.1: three files get headers with blank line separators
		{
			Name:     "R3.1_three_file_headers",
			Args:     []string{hf1, hf2, hf3},
			ExitCode: 0,
		},
		// R3.2: single file no header
		{
			Name:     "R3.2_single_file_no_header",
			Args:     []string{hf1},
			ExitCode: 0,
		},
		// R3.3: -q suppresses headers for multiple files
		{
			Name:     "R3.3_quiet_multi_file",
			Args:     []string{"-q", hf1, hf2},
			ExitCode: 0,
		},
		// R3.3: --quiet suppresses headers
		{
			Name:     "R3.3_quiet_long_flag",
			Args:     []string{"--quiet", hf1, hf2},
			ExitCode: 0,
		},
		// R3.3: --silent suppresses headers
		{
			Name:     "R3.3_silent_flag",
			Args:     []string{"--silent", hf1, hf2},
			ExitCode: 0,
		},
		// R3.4: -v forces header for single file
		{
			Name:     "R3.4_verbose_single_file",
			Args:     []string{"-v", hf1},
			ExitCode: 0,
		},
		// R3.4: --verbose forces header for single file
		{
			Name:     "R3.4_verbose_long_flag",
			Args:     []string{"--verbose", hf1},
			ExitCode: 0,
		},
	}
}

// buildByteTests returns differential tests for R2.1, R2.2, R2.3 (byte mode).
func buildByteTests(t *testing.T) []testutils.DiffTest {
	t.Helper()

	// Fixed byte input for predictable byte offsets.
	alphaBytes := []byte("abcdefghijklmnopqrstuvwxyz")
	input20 := generateLines(20)

	byteDir := t.TempDir()
	byteFile := createTestFile(t, byteDir, "alpha.txt", string(alphaBytes))

	return []testutils.DiffTest{
		// R2.1: -c 5 last 5 bytes from stdin
		{
			Name:     "c_5_bytes",
			Args:     []string{"-c", "5"},
			Stdin:    alphaBytes,
			ExitCode: 0,
		},
		// R2.1: --bytes=5
		{
			Name:     "bytes_eq_5",
			Args:     []string{"--bytes=5"},
			Stdin:    alphaBytes,
			ExitCode: 0,
		},
		// R2.1: -c5 no space
		{
			Name:     "c5_no_space",
			Args:     []string{"-c5"},
			Stdin:    alphaBytes,
			ExitCode: 0,
		},
		// R2.1: -c 0 prints nothing
		{
			Name:     "c_0_bytes",
			Args:     []string{"-c", "0"},
			Stdin:    alphaBytes,
			ExitCode: 0,
		},
		// R2.1: -c more than input length prints all
		{
			Name:     "c_more_than_input",
			Args:     []string{"-c", "100"},
			Stdin:    alphaBytes,
			ExitCode: 0,
		},
		// R2.1: -c and -n mutual exclusivity, last wins (R2.1)
		{
			Name:     "c_after_n_last_wins",
			Args:     []string{"-n", "3", "-c", "5"},
			Stdin:    alphaBytes,
			ExitCode: 0,
		},
		// R2.1: -n after -c, last wins
		{
			Name:     "n_after_c_last_wins",
			Args:     []string{"-c", "5", "-n", "3"},
			Stdin:    input20,
			ExitCode: 0,
		},
		// R2.1: -c on file argument
		{
			Name:     "c_on_file",
			Args:     []string{"-c", "10", byteFile},
			ExitCode: 0,
		},
		// R2.2: -c +5 from byte offset
		{
			Name:     "c_plus_5_offset",
			Args:     []string{"-c", "+5"},
			Stdin:    alphaBytes,
			ExitCode: 0,
		},
		// R2.2: -c +1 prints entire input
		{
			Name:     "c_plus_1_all",
			Args:     []string{"-c", "+1"},
			Stdin:    alphaBytes,
			ExitCode: 0,
		},
		// R2.2: -c +N beyond input length
		{
			Name:     "c_plus_beyond_input",
			Args:     []string{"-c", "+100"},
			Stdin:    alphaBytes,
			ExitCode: 0,
		},
		// R2.3: -c with b suffix (512-byte blocks)
		{
			Name:     "c_suffix_b",
			Args:     []string{"-c", "1b"},
			Stdin:    alphaBytes,
			ExitCode: 0,
		},
		// R2.3: -c with K suffix (1024 bytes)
		{
			Name:     "c_suffix_K",
			Args:     []string{"-c", "1K"},
			Stdin:    alphaBytes,
			ExitCode: 0,
		},
	}
}
