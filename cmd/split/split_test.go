// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd067-split R1.1–R1.4.
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareSplitOutput(t, goBin, refBin, tc)
		})
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
	t.Fatalf("binary %s failed: %v", bin, err)
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

// truncate returns at most 256 bytes for display.
func truncate(data []byte) []byte {
	if len(data) <= 256 {
		return data
	}
	return data[:256]
}
