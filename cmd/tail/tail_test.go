// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tail against gtail (GNU coreutils).
//
// Covers prd055-tail R1.1, R1.2, R1.3, R1.4: default 10 lines, explicit -n
// count, --lines= form, +N offset, stdin input, stdin via "-" argument.
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

// buildDiffTests returns all differential test cases for tail R1.1-R1.4.
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
