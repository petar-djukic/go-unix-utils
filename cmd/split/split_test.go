// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd067-split R1.1–R1.4, R2.1–R2.4, R3.1–R3.4.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const testTimeout = 10 * time.Second

// splitTest defines a differential test case for split.
type splitTest struct {
	name      string
	args      []string
	stdin     []byte
	inputFile string // create this file in work dir with inputData
	inputData []byte
}

// seqLines generates "1\n2\n...\nN\n".
func seqLines(n int) []byte {
	var buf bytes.Buffer
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&buf, "%d\n", i)
	}
	return buf.Bytes()
}

// repeatBytes generates a byte slice of length n filled with 'A'.
func repeatBytes(n int) []byte {
	return bytes.Repeat([]byte("A"), n)
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsplit")
	if err != nil {
		t.Skipf("reference binary gsplit not in PATH: %v", err)
	}

	tests := []splitTest{
		// R1.1: default 1000-line split from stdin
		{
			name:  "default_1000_lines",
			stdin: seqLines(3000),
		},
		// R1.1: single chunk (fewer than 1000 lines)
		{
			name:  "single_chunk",
			stdin: seqLines(500),
		},
		// R1.1: exact multiple of default line count
		{
			name:  "exact_2000_lines",
			stdin: seqLines(2000),
		},
		// R1.1: empty input produces no files
		{
			name:  "empty_input",
			stdin: []byte{},
		},
		// R1.1: input without trailing newline
		{
			name:  "no_trailing_newline",
			args:  []string{"-l", "2"},
			stdin: []byte("a\nb\nc"),
		},
		// R1.2: custom prefix
		{
			name:  "custom_prefix",
			args:  []string{"-", "chunk"},
			stdin: seqLines(5),
		},
		// R1.2: custom prefix with line option
		{
			name:  "prefix_and_lines",
			args:  []string{"-l", "2", "-", "out_"},
			stdin: seqLines(7),
		},
		// R1.3: -l with separate argument
		{
			name:  "dash_l_separate",
			args:  []string{"-l", "3"},
			stdin: seqLines(10),
		},
		// R1.3: -l with attached number
		{
			name:  "dash_l_attached",
			args:  []string{"-l5"},
			stdin: seqLines(12),
		},
		// R1.3: --lines= form
		{
			name:  "lines_equals",
			args:  []string{"--lines=4"},
			stdin: seqLines(9),
		},
		// R1.3: --lines with space-separated value
		{
			name:  "lines_space",
			args:  []string{"--lines", "4"},
			stdin: seqLines(9),
		},
		// R1.3: one line per file
		{
			name:  "one_line_per_file",
			args:  []string{"-l", "1"},
			stdin: seqLines(5),
		},
		// R1.4: stdin explicitly via -
		{
			name:  "stdin_via_dash",
			args:  []string{"-l", "3", "-"},
			stdin: seqLines(10),
		},
		// R1.4: file input
		{
			name:      "file_input",
			args:      []string{"-l", "3", "input.txt"},
			inputFile: "input.txt",
			inputData: seqLines(10),
		},
		// R1.4: file input with custom prefix
		{
			name:      "file_input_prefix",
			args:      []string{"-l", "5", "input.txt", "part_"},
			inputFile: "input.txt",
			inputData: seqLines(12),
		},
		// R2.1: -b byte split
		{
			name:  "bytes_100",
			args:  []string{"-b", "100"},
			stdin: repeatBytes(350),
		},
		// R2.1: -b with attached value
		{
			name:  "bytes_attached",
			args:  []string{"-b50"},
			stdin: repeatBytes(120),
		},
		// R2.1: --bytes= form
		{
			name:  "bytes_equals",
			args:  []string{"--bytes=75"},
			stdin: repeatBytes(200),
		},
		// R2.1: -b with K suffix (1024)
		{
			name:      "bytes_K_suffix",
			args:      []string{"-b", "1K", "input.bin"},
			inputFile: "input.bin",
			inputData: repeatBytes(3000),
		},
		// R2.1: -b exact fit
		{
			name:  "bytes_exact_fit",
			args:  []string{"-b", "50"},
			stdin: repeatBytes(100),
		},
		// R2.1: -b larger than input
		{
			name:  "bytes_larger_than_input",
			args:  []string{"-b", "500"},
			stdin: repeatBytes(100),
		},
		// R2.2: -C line-bytes basic
		{
			name:  "line_bytes_basic",
			args:  []string{"-C", "20"},
			stdin: []byte("short\nmedium line\nthis is a longer line\nend\n"),
		},
		// R2.2: -C with --line-bytes= form
		{
			name:  "line_bytes_equals",
			args:  []string{"--line-bytes=15"},
			stdin: []byte("hello\nworld\nfoo\nbar\nbaz\n"),
		},
		// R2.2: -C line longer than limit
		{
			name:  "line_bytes_long_line",
			args:  []string{"-C", "5"},
			stdin: []byte("abcdefghij\nxy\n"),
		},
		// R2.2: -C from file
		{
			name:      "line_bytes_file",
			args:      []string{"-C", "30", "input.txt"},
			inputFile: "input.txt",
			inputData: seqLines(20),
		},
		// R2.3: -n N (split into N byte-based chunks)
		{
			name:  "chunks_bytes_3",
			args:  []string{"-n", "3"},
			stdin: repeatBytes(100),
		},
		// R2.3: -n N with file input
		{
			name:      "chunks_bytes_file",
			args:      []string{"-n", "4", "input.bin"},
			inputFile: "input.bin",
			inputData: repeatBytes(100),
		},
		// R2.3: -n l/N (split by lines into N chunks)
		{
			name:  "chunks_lines_3",
			args:  []string{"-n", "l/3"},
			stdin: seqLines(10),
		},
		// R2.3: -n r/N (round-robin by lines)
		{
			name:  "chunks_roundrobin_3",
			args:  []string{"-n", "r/3"},
			stdin: seqLines(10),
		},
		// R2.3: -n r/N with file
		{
			name:      "chunks_roundrobin_file",
			args:      []string{"-n", "r/2", "input.txt"},
			inputFile: "input.txt",
			inputData: seqLines(7),
		},
		// R2.3: --number= form
		{
			name:  "number_equals",
			args:  []string{"--number=2"},
			stdin: repeatBytes(50),
		},
		// R3.1: -a suffix length
		{
			name:  "suffix_length_3",
			args:  []string{"-a", "3", "-l", "2"},
			stdin: seqLines(7),
		},
		// R3.1: --suffix-length= form
		{
			name:  "suffix_length_equals",
			args:  []string{"--suffix-length=4", "-l", "3"},
			stdin: seqLines(10),
		},
		// R3.1: -a with attached value
		{
			name:  "suffix_length_attached",
			args:  []string{"-a3", "-l", "2"},
			stdin: seqLines(5),
		},
		// R3.2: -d numeric suffixes
		{
			name:  "numeric_suffixes",
			args:  []string{"-d", "-l", "3"},
			stdin: seqLines(10),
		},
		// R3.2: --numeric-suffixes long form
		{
			name:  "numeric_suffixes_long",
			args:  []string{"--numeric-suffixes", "-l", "5"},
			stdin: seqLines(12),
		},
		// R3.2: -d with custom suffix length
		{
			name:  "numeric_suffix_length",
			args:  []string{"-d", "-a", "3", "-l", "2"},
			stdin: seqLines(7),
		},
		// R3.2: -d with custom prefix
		{
			name:  "numeric_prefix",
			args:  []string{"-d", "-l", "3", "-", "chunk"},
			stdin: seqLines(8),
		},
		// R3.3: --additional-suffix
		{
			name:  "additional_suffix",
			args:  []string{"--additional-suffix=.txt", "-l", "3"},
			stdin: seqLines(8),
		},
		// R3.3: --additional-suffix with numeric
		{
			name:  "additional_suffix_numeric",
			args:  []string{"--additional-suffix=.dat", "-d", "-l", "5"},
			stdin: seqLines(12),
		},
		// R3.3: --additional-suffix with custom prefix
		{
			name:  "additional_suffix_prefix",
			args:  []string{"--additional-suffix=.log", "-l", "4", "-", "out_"},
			stdin: seqLines(10),
		},
		// R3.1+R3.2+R3.3 combined
		{
			name:  "all_suffix_options",
			args:  []string{"-d", "-a", "3", "--additional-suffix=.csv", "-l", "2"},
			stdin: seqLines(7),
		},
		// R3.2: -d with byte splitting
		{
			name:  "numeric_bytes",
			args:  []string{"-d", "-b", "50"},
			stdin: repeatBytes(120),
		},
		// R3.1: suffix length with chunks
		{
			name:  "suffix_length_chunks",
			args:  []string{"-a", "3", "-n", "3"},
			stdin: repeatBytes(100),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareSplitOutput(t, goBin, refBin, tc)
		})
	}
}

// TestConflictingModes verifies R2.4: conflicting split options produce error.
func TestConflictingModes(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsplit")
	if err != nil {
		t.Skipf("reference binary gsplit not in PATH: %v", err)
	}

	tests := []splitTest{
		{
			name:  "conflict_bytes_lines",
			args:  []string{"-b", "100", "-l", "10"},
			stdin: seqLines(20),
		},
		{
			name:  "conflict_bytes_chunks",
			args:  []string{"-b", "100", "-n", "3"},
			stdin: seqLines(20),
		},
		{
			name:  "conflict_linebytes_lines",
			args:  []string{"-C", "100", "-l", "10"},
			stdin: seqLines(20),
		},
		{
			name:  "conflict_linebytes_chunks",
			args:  []string{"-C", "100", "-n", "3"},
			stdin: seqLines(20),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareExitCodes(t, goBin, refBin, tc)
		})
	}
}

// compareExitCodes runs both binaries and verifies both exit non-zero.
func compareExitCodes(t *testing.T, goBin, refBin string, tc splitTest) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	setupInputFile(t, refDir, tc)
	setupInputFile(t, goDir, tc)
	refExit := runSplitBinary(t, refBin, tc.args, tc.stdin, refDir)
	goExit := runSplitBinary(t, goBin, tc.args, tc.stdin, goDir)
	if refExit == 0 {
		t.Skipf("reference binary did not fail for %s", tc.name)
	}
	if goExit == 0 {
		t.Fatalf("expected non-zero exit, got 0")
	}
}

// compareSplitOutput runs both binaries in separate directories and
// compares all output files and exit codes.
func compareSplitOutput(t *testing.T, goBin, refBin string, tc splitTest) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	setupInputFile(t, refDir, tc)
	setupInputFile(t, goDir, tc)
	refExit := runSplitBinary(t, refBin, tc.args, tc.stdin, refDir)
	goExit := runSplitBinary(t, goBin, tc.args, tc.stdin, goDir)
	if refExit != goExit {
		t.Fatalf("exit code divergence: ref=%d go=%d", refExit, goExit)
	}
	compareOutputFiles(t, refDir, goDir, tc.inputFile)
}

// setupInputFile creates the input file in dir if the test requires one.
func setupInputFile(t *testing.T, dir string, tc splitTest) {
	t.Helper()
	if tc.inputFile == "" {
		return
	}
	path := filepath.Join(dir, tc.inputFile)
	if err := os.WriteFile(path, tc.inputData, 0o644); err != nil {
		t.Fatalf("setup input file: %v", err)
	}
}

// runSplitBinary executes a binary in the given directory and returns exit code.
func runSplitBinary(t *testing.T, bin string, args []string, stdin []byte, dir string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", bin)
	}
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	t.Fatalf("binary %s failed: %v\nstderr: %s", bin, err, stderr.String())
	return -1
}

// compareOutputFiles compares all non-input files between two directories.
func compareOutputFiles(t *testing.T, refDir, goDir, inputFile string) {
	t.Helper()
	refFiles := listOutputFiles(t, refDir, inputFile)
	goFiles := listOutputFiles(t, goDir, inputFile)
	if !equalStringSlices(refFiles, goFiles) {
		t.Fatalf("file list divergence\nref: %v\ngo:  %v", refFiles, goFiles)
	}
	for _, name := range refFiles {
		compareFileContents(t, refDir, goDir, name)
	}
}

// listOutputFiles returns sorted filenames in dir, excluding inputFile.
func listOutputFiles(t *testing.T, dir, inputFile string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if e.Name() != inputFile {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// equalStringSlices returns true if two string slices are equal.
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

// compareFileContents compares a named file in two directories.
func compareFileContents(t *testing.T, dir1, dir2, name string) {
	t.Helper()
	data1, err := os.ReadFile(filepath.Join(dir1, name))
	if err != nil {
		t.Fatalf("read ref file %s: %v", name, err)
	}
	data2, err := os.ReadFile(filepath.Join(dir2, name))
	if err != nil {
		t.Fatalf("read go file %s: %v", name, err)
	}
	if !bytes.Equal(data1, data2) {
		t.Fatalf("file %s content divergence\nref (%d bytes): %q\ngo  (%d bytes): %q",
			name, len(data1), truncate(data1), len(data2), truncate(data2))
	}
}

// TestFilter verifies R3.4: --filter pipes output to a shell command.
func TestFilter(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsplit")
	if err != nil {
		t.Skipf("reference binary gsplit not in PATH: %v", err)
	}

	tests := []splitTest{
		// R3.4: --filter with cat piping to file
		{
			name:  "filter_cat",
			args:  []string{"--filter=cat > $FILE.filtered", "-l", "3"},
			stdin: seqLines(7),
		},
		// R3.4: --filter with numeric suffix
		{
			name:  "filter_numeric",
			args:  []string{"--filter=cat > $FILE.out", "-d", "-l", "4"},
			stdin: seqLines(10),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareSplitOutput(t, goBin, refBin, tc)
		})
	}
}

// truncate returns at most 256 bytes for display.
func truncate(data []byte) []byte {
	if len(data) <= 256 {
		return data
	}
	return data[:256]
}
