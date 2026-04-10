// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/split via differential testing against gsplit.
// Implements srd067-split R4.3, R4.4 for requirements R1.1-R1.4.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// splitTest defines a test case for split file-output comparison.
type splitTest struct {
	name  string
	args  []string
	stdin []byte
}

// TestDiff runs differential tests comparing Go split against gsplit.
// R4.3: compares output file contents and exit codes.
// R4.4: covers default 1000-line split, -l custom, custom prefix, stdin.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsplit")
	if err != nil {
		t.Skipf("reference binary gsplit not in PATH: %v", err)
	}
	tests := []splitTest{
		{
			name:  "default_small_input",
			stdin: numberedLines(1, 5),
		},
		{
			name:  "lines_3",
			args:  []string{"-l", "3"},
			stdin: numberedLines(1, 7),
		},
		{
			name:  "lines_long_form",
			args:  []string{"--lines=2"},
			stdin: []byte("x\ny\nz\n"),
		},
		{
			name:  "custom_prefix",
			args:  []string{"-l", "2", "-", "chunk_"},
			stdin: []byte("a\nb\nc\n"),
		},
		{
			name:  "stdin_explicit_dash",
			args:  []string{"-l", "2", "-"},
			stdin: []byte("1\n2\n3\n"),
		},
		{
			name:  "lines_attached_value",
			args:  []string{"-l2"},
			stdin: []byte("a\nb\nc\nd\ne\n"),
		},
		{
			name:  "single_line_input",
			args:  []string{"-l", "1"},
			stdin: []byte("only\n"),
		},
		{
			name:  "no_trailing_newline",
			args:  []string{"-l", "2"},
			stdin: []byte("a\nb\nc"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			compareSplitOutputs(t, goBin, refBin, tc.args, tc.stdin)
		})
	}
}

// TestDiffExitCodes uses RunDiffTests for exit code and stderr comparison.
func TestDiffExitCodes(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsplit")
	if err != nil {
		t.Skipf("reference binary gsplit not in PATH: %v", err)
	}
	norm := []testutils.NormalizeFunc{stderrProgNormalizer}
	tests := []testutils.DiffTest{
		{
			Name:      "conflicting_options",
			Args:      []string{"-l", "10", "-b", "100"},
			Stdin:     []byte("data\n"),
			ExitCode:  1,
			Normalize: norm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// compareSplitOutputs runs both binaries in separate dirs and compares files.
func compareSplitOutputs(t *testing.T, goBin, refBin string, args []string, stdin []byte) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	refCode := runInDir(t, refBin, args, stdin, refDir)
	goCode := runInDir(t, goBin, args, stdin, goDir)
	if refCode != goCode {
		t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
	}
	refFiles := collectFiles(t, refDir)
	goFiles := collectFiles(t, goDir)
	compareFileMaps(t, refFiles, goFiles)
}

// runInDir executes a binary in the given directory and returns the exit code.
func runInDir(t *testing.T, bin string, args []string, stdin []byte, dir string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	return extractExitCode(t, cmd)
}

// extractExitCode runs the command and returns its exit code.
func extractExitCode(t *testing.T, cmd *exec.Cmd) int {
	t.Helper()
	err := cmd.Run()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	t.Fatalf("binary %s failed to execute: %v", cmd.Path, err)
	return -1
}

// collectFiles reads all regular files in dir and returns name→content.
func collectFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	result := make(map[string][]byte)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading file %s: %v", e.Name(), err)
		}
		result[e.Name()] = data
	}
	return result
}

// compareFileMaps verifies that ref and go produced identical file sets.
func compareFileMaps(t *testing.T, ref, got map[string][]byte) {
	t.Helper()
	allNames := mergeKeys(ref, got)
	for _, name := range allNames {
		refData, inRef := ref[name]
		goData, inGo := got[name]
		if inRef && !inGo {
			t.Errorf("file %q: present in ref but missing in go", name)
			continue
		}
		if !inRef && inGo {
			t.Errorf("file %q: present in go but missing in ref", name)
			continue
		}
		if !bytes.Equal(refData, goData) {
			t.Errorf("file %q: content mismatch\n  ref len=%d\n  go  len=%d",
				name, len(refData), len(goData))
		}
	}
}

// mergeKeys returns the sorted union of keys from two maps.
func mergeKeys(a, b map[string][]byte) []string {
	seen := make(map[string]bool)
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// progNameRe matches the binary name prefix in stderr error messages.
var progNameRe = regexp.MustCompile(`^[^\s:]+:\s`)

// tryHelpRe matches the "Try ... --help" line GNU appends to errors.
var tryHelpRe = regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)

// stderrProgNormalizer strips program name prefixes and GNU "Try --help" lines
// so that stderr comparison ignores expected formatting differences.
var stderrProgNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	b = tryHelpRe.ReplaceAll(b, nil)
	b = progNameRe.ReplaceAll(b, []byte("PROG: "))
	return b
}

// numberedLines generates lines "1\n2\n...\nto\n".
func numberedLines(from, to int) []byte {
	var buf bytes.Buffer
	for i := from; i <= to; i++ {
		fmt.Fprintf(&buf, "%d\n", i)
	}
	return buf.Bytes()
}
