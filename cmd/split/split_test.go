// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd067-split R1.1-R1.4, R2.1-R2.4, R3.1-R3.3, R4.1-R4.4.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes split/gsplit stderr differences:
// replaces the binary name prefix and removes "Try ... --help" lines.
func stderrNormalizer(data []byte) []byte {
	// Normalize binary name: gsplit → split
	s := strings.ReplaceAll(string(data), "gsplit:", "split:")
	// Remove "Try '...' --help" hint lines
	re := regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)
	s = re.ReplaceAllString(s, "")
	return []byte(s)
}

// TestDiff builds the split binary and compares output against gsplit.
// R4.1-R4.4: uses testutils.BuildBinary, exec.LookPath("gsplit"),
// and testutils.RunDiffTests for exit-code/stderr tests.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsplit")
	if err != nil {
		t.Skip("reference binary gsplit not in PATH")
	}

	t.Run("error_cases", func(t *testing.T) {
		runErrorTests(t, goBin, refBin)
	})
	t.Run("file_output", func(t *testing.T) {
		runFileOutputTests(t, goBin, refBin)
	})
}

// runErrorTests uses testutils.RunDiffTests for cases that only
// produce stderr/exit code output (no output files to compare).
// R4.2: invalid option, conflicting options.
func runErrorTests(t *testing.T, goBin, refBin string) {
	t.Helper()
	norm := []testutils.NormalizeFunc{stderrNormalizer}
	tests := []testutils.DiffTest{
		{
			Name:      "conflicting_bytes_and_lines",
			Args:      []string{"-b", "10", "-l", "5"},
			Normalize: norm,
		},
		{
			Name:      "conflicting_chunks_and_bytes",
			Args:      []string{"-n", "3", "-b", "10"},
			Normalize: norm,
		},
		{
			Name:      "conflicting_lines_and_chunks",
			Args:      []string{"-l", "5", "-n", "3"},
			Normalize: norm,
		},
		{
			Name:      "invalid_line_count_zero",
			Args:      []string{"-l", "0"},
			Normalize: norm,
		},
		{
			Name:      "invalid_line_count_negative",
			Args:      []string{"-l", "-1"},
			Normalize: norm,
		},
		{
			Name:      "invalid_byte_count",
			Args:      []string{"-b", "abc"},
			Normalize: norm,
		},
		{
			Name:      "invalid_chunk_count_zero",
			Args:      []string{"-n", "0"},
			Normalize: norm,
		},
		{
			Name:      "extra_operand",
			Args:      []string{"file1", "prefix", "extra"},
			Normalize: norm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runFileOutputTests exercises split modes that produce output files,
// comparing files between separate reference and Go temp directories.
func runFileOutputTests(t *testing.T, goBin, refBin string) {
	t.Helper()
	tests := []struct {
		name  string
		args  []string
		stdin string
	}{
		// R1.1: default 1000-line split
		{
			name:  "default_1000_lines",
			stdin: generateLines(2500),
		},
		// R1.3: custom line count
		{
			name:  "lines_3",
			args:  []string{"-l", "3"},
			stdin: generateLines(10),
		},
		// R1.3: long-form --lines=
		{
			name:  "lines_long_form",
			args:  []string{"--lines=5"},
			stdin: generateLines(12),
		},
		// R2.1: byte-based split
		{
			name:  "bytes_50",
			args:  []string{"-b", "50"},
			stdin: strings.Repeat("abcdefghij", 20),
		},
		// R2.1: byte split with size suffix
		{
			name:  "bytes_1K_suffix",
			args:  []string{"-b", "1K"},
			stdin: strings.Repeat("x", 3000),
		},
		// R2.2: line-bytes mode
		{
			name:  "line_bytes_30",
			args:  []string{"-C", "30"},
			stdin: generateLines(20),
		},
		// R2.2: line-bytes with long line exceeding limit
		{
			name:  "line_bytes_long_line",
			args:  []string{"-C", "10"},
			stdin: "short\n" + strings.Repeat("a", 25) + "\nend\n",
		},
		// R1.2: custom prefix
		{
			name:  "custom_prefix",
			args:  []string{"-l", "5", "-", "chunk_"},
			stdin: generateLines(12),
		},
		// R1.3: single line per file
		{
			name:  "single_line_per_file",
			args:  []string{"-l", "1"},
			stdin: "one\ntwo\nthree\n",
		},
		// R1.4: explicit stdin dash
		{
			name:  "stdin_dash",
			args:  []string{"-l", "2", "-"},
			stdin: "a\nb\nc\nd\n",
		},
		// R2.3: chunk mode by bytes (-n N)
		{
			name:  "chunks_bytes_3",
			args:  []string{"-n", "3"},
			stdin: strings.Repeat("x", 30),
		},
		// R2.3: chunk mode by lines (-n l/N)
		{
			name:  "chunks_lines_3",
			args:  []string{"-n", "l/3"},
			stdin: generateLines(10),
		},
		// R2.3: round-robin mode (-n r/N)
		{
			name:  "chunks_round_robin_3",
			args:  []string{"-n", "r/3"},
			stdin: generateLines(9),
		},
		// R2.3: round-robin with uneven distribution
		{
			name:  "chunks_round_robin_uneven",
			args:  []string{"-n", "r/4"},
			stdin: generateLines(7),
		},
		// R3.1: suffix length
		{
			name:  "suffix_length_3",
			args:  []string{"-a", "3", "-l", "2"},
			stdin: generateLines(6),
		},
		// R3.2: numeric suffixes
		{
			name:  "numeric_suffixes",
			args:  []string{"-d", "-l", "2"},
			stdin: generateLines(6),
		},
		// R3.1 + R3.2: numeric suffixes with custom length
		{
			name:  "numeric_suffix_length_3",
			args:  []string{"-d", "-a", "3", "-l", "3", "-", "prefix_"},
			stdin: generateLines(9),
		},
		// R3.3: additional suffix
		{
			name:  "additional_suffix",
			args:  []string{"--additional-suffix=.txt", "-l", "3"},
			stdin: generateLines(9),
		},
		// R3.3: additional suffix with numeric
		{
			name:  "additional_suffix_numeric",
			args:  []string{"-d", "--additional-suffix=.dat", "-l", "2"},
			stdin: generateLines(6),
		},
		// Edge: empty input
		{
			name:  "empty_input",
			args:  []string{"-l", "10"},
			stdin: "",
		},
		// Edge: single byte input
		{
			name:  "single_byte",
			args:  []string{"-b", "1"},
			stdin: "x",
		},
		// Edge: no trailing newline
		{
			name:  "no_trailing_newline",
			args:  []string{"-l", "2"},
			stdin: "line1\nline2\nline3",
		},
		// Edge: very long line with line split
		{
			name:  "very_long_line",
			args:  []string{"-l", "1"},
			stdin: strings.Repeat("a", 5000) + "\nb\n",
		},
		// Edge: binary content with byte split
		{
			name:  "binary_content_bytes",
			args:  []string{"-b", "16"},
			stdin: string(binaryContent(64)),
		},
		// Edge: binary content with line split
		{
			name:  "binary_content_lines",
			args:  []string{"-l", "1"},
			stdin: "line1\n\x00\x01\x02\nline3\n",
		},
		// Edge: chunk byte split on uneven size
		{
			name:  "chunks_bytes_uneven",
			args:  []string{"-n", "4"},
			stdin: strings.Repeat("a", 13),
		},
		// Edge: chunk line split with equal lines and chunks
		{
			name:  "chunks_lines_equal",
			args:  []string{"-n", "l/3"},
			stdin: generateLines(3),
		},
		// Edge: single chunk
		{
			name:  "single_chunk",
			args:  []string{"-n", "1"},
			stdin: generateLines(5),
		},
		// Edge: bytes split exactly divisible
		{
			name:  "bytes_exact_divisible",
			args:  []string{"-b", "10"},
			stdin: strings.Repeat("a", 30),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runAndCompare(t, goBin, refBin, tc.args, tc.stdin)
		})
	}
}

// generateLines produces a string with n numbered lines.
func generateLines(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "%d\n", i)
	}
	return b.String()
}

// binaryContent produces n bytes of non-text content including nulls.
func binaryContent(n int) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
}

// runAndCompare runs both binaries in separate temp dirs and compares
// output files, stderr, and exit codes.
func runAndCompare(
	t *testing.T, goBin, refBin string, args []string, stdin string,
) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	refExit, refStderr := runSplit(t, refBin, args, stdin, refDir)
	goExit, goStderr := runSplit(t, goBin, args, stdin, goDir)

	if refExit != goExit {
		t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
	}
	if refStderr != goStderr {
		t.Logf("stderr diff: ref=%q go=%q", refStderr, goStderr)
	}

	compareOutputFiles(t, refDir, goDir)
}

// runSplit executes a split binary in the given directory and returns
// exit code and stderr output.
func runSplit(
	t *testing.T, bin string, args []string, stdin, dir string,
) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", bin, err)
		}
	}

	return exitCode, stderr.String()
}

// compareOutputFiles compares all files between two directories.
func compareOutputFiles(t *testing.T, refDir, goDir string) {
	t.Helper()
	refFiles := listOutputFiles(t, refDir)
	goFiles := listOutputFiles(t, goDir)

	if !equalStringSlices(refFiles, goFiles) {
		t.Errorf("file lists differ\n  ref: %v\n  go:  %v",
			refFiles, goFiles)
		return
	}

	for _, name := range refFiles {
		compareFileContents(t, name, refDir, goDir)
	}
}

// listOutputFiles returns sorted filenames in a directory.
func listOutputFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// compareFileContents checks that a named file has identical content
// in both directories.
func compareFileContents(
	t *testing.T, name, refDir, goDir string,
) {
	t.Helper()
	refData, err := os.ReadFile(filepath.Join(refDir, name))
	if err != nil {
		t.Errorf("read ref %s: %v", name, err)
		return
	}
	goData, err := os.ReadFile(filepath.Join(goDir, name))
	if err != nil {
		t.Errorf("read go %s: %v", name, err)
		return
	}
	if !bytes.Equal(refData, goData) {
		t.Errorf("file %s differs\n  ref: %q\n  go:  %q", name,
			truncate(refData, 200), truncate(goData, 200))
	}
}

// equalStringSlices returns true if two string slices are identical.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// truncate returns at most n bytes of data for display.
func truncate(data []byte, n int) []byte {
	if len(data) <= n {
		return data
	}
	return data[:n]
}
