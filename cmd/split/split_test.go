// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd067-split R1.1-R1.4, R2.1-R2.2.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff builds the split binary and compares output against gsplit.
// AC4: uses testutils.BuildBinary and exec.LookPath("gsplit").
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsplit")
	if err != nil {
		t.Skip("reference binary gsplit not in PATH")
	}

	tests := []struct {
		name  string
		args  []string
		stdin string
	}{
		{
			name:  "default_1000_lines",
			stdin: generateLines(2500),
		},
		{
			name:  "lines_3",
			args:  []string{"-l", "3"},
			stdin: generateLines(10),
		},
		{
			name:  "lines_long_form",
			args:  []string{"--lines=5"},
			stdin: generateLines(12),
		},
		{
			name:  "bytes_50",
			args:  []string{"-b", "50"},
			stdin: strings.Repeat("abcdefghij", 20),
		},
		{
			name:  "bytes_1K_suffix",
			args:  []string{"-b", "1K"},
			stdin: strings.Repeat("x", 3000),
		},
		{
			name:  "line_bytes_30",
			args:  []string{"-C", "30"},
			stdin: generateLines(20),
		},
		{
			name:  "line_bytes_long_line",
			args:  []string{"-C", "10"},
			stdin: "short\n" + strings.Repeat("a", 25) + "\nend\n",
		},
		{
			name:  "custom_prefix",
			args:  []string{"-l", "5", "-", "chunk_"},
			stdin: generateLines(12),
		},
		{
			name:  "single_line",
			args:  []string{"-l", "1"},
			stdin: "one\ntwo\nthree\n",
		},
		{
			name:  "stdin_dash",
			args:  []string{"-l", "2", "-"},
			stdin: "a\nb\nc\nd\n",
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

// compareOutputFiles compares all non-input files between two directories.
func compareOutputFiles(t *testing.T, refDir, goDir string) {
	t.Helper()
	refFiles := listOutputFiles(t, refDir)
	goFiles := listOutputFiles(t, goDir)

	if !equalStringSlices(refFiles, goFiles) {
		t.Errorf("file lists differ\n  ref: %v\n  go:  %v", refFiles, goFiles)
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
func compareFileContents(t *testing.T, name, refDir, goDir string) {
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
