// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/split via differential testing against gsplit.
// Implements srd067-split R4.3, R4.4 for requirements R1.1-R1.4, R2.1-R2.4, R3.1-R3.4.
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
	"strings"
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
// R4.4: covers default 1000-line split, -l custom, custom prefix, stdin,
// -b byte split, -C line-bytes, -n chunk modes.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsplit")
	if err != nil {
		t.Skipf("reference binary gsplit not in PATH: %v", err)
	}
	tests := []splitTest{
		// R1 tests (line-based splitting)
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
		// R2.1 tests (byte-based splitting)
		{
			name:  "bytes_5",
			args:  []string{"-b", "5"},
			stdin: []byte("hello world this is test\n"),
		},
		{
			name:  "bytes_long_form",
			args:  []string{"--bytes=10"},
			stdin: []byte("abcdefghijklmnopqrstuvwxyz\n"),
		},
		{
			name:  "bytes_exact_boundary",
			args:  []string{"-b", "3"},
			stdin: []byte("abcdef"),
		},
		{
			name:  "bytes_larger_than_input",
			args:  []string{"-b", "100"},
			stdin: []byte("small\n"),
		},
		// R2.2 tests (line-bytes splitting)
		{
			name:  "line_bytes_10",
			args:  []string{"-C", "10"},
			stdin: []byte("short\nhi\nworld\ntest\n"),
		},
		{
			name:  "line_bytes_long_line",
			args:  []string{"-C", "5"},
			stdin: []byte("ab\nabcdefghij\nxy\n"),
		},
		{
			name:  "line_bytes_exact_fit",
			args:  []string{"-C", "4"},
			stdin: []byte("abc\ndef\nghi\n"),
		},
		// R2.3 tests (chunk-based splitting)
		{
			name:  "chunks_3_bytes",
			args:  []string{"-n", "3"},
			stdin: numberedLines(1, 9),
		},
		{
			name:  "chunks_line_mode",
			args:  []string{"-n", "l/3"},
			stdin: numberedLines(1, 9),
		},
		{
			name:  "chunks_round_robin",
			args:  []string{"-n", "r/3"},
			stdin: numberedLines(1, 9),
		},
		{
			name:  "chunks_2_small_input",
			args:  []string{"-n", "2"},
			stdin: []byte("abcde"),
		},
		{
			name:  "chunks_more_than_bytes",
			args:  []string{"-n", "5"},
			stdin: []byte("ab"),
		},
		// R3.1 tests (suffix length)
		{
			name:  "suffix_length_3",
			args:  []string{"-a", "3", "-l", "2"},
			stdin: []byte("a\nb\nc\nd\ne\n"),
		},
		{
			name:  "suffix_length_1",
			args:  []string{"-a", "1", "-l", "1"},
			stdin: []byte("a\nb\nc\n"),
		},
		// R3.2 tests (numeric suffixes)
		{
			name:  "numeric_suffixes",
			args:  []string{"-d", "-l", "2"},
			stdin: []byte("a\nb\nc\nd\ne\n"),
		},
		{
			name:  "numeric_suffix_length_3",
			args:  []string{"-d", "-a", "3", "-l", "2"},
			stdin: []byte("a\nb\nc\n"),
		},
		// R3.3 tests (additional suffix)
		{
			name:  "additional_suffix_txt",
			args:  []string{"--additional-suffix=.txt", "-l", "2"},
			stdin: []byte("a\nb\nc\n"),
		},
		{
			name:  "additional_suffix_with_numeric",
			args:  []string{"--additional-suffix=.dat", "-d", "-l", "1"},
			stdin: []byte("x\ny\n"),
		},
		// R3.4 tests (filter)
		{
			name:  "filter_cat_to_file",
			args:  []string{"--filter=cat > $FILE", "-l", "2"},
			stdin: []byte("a\nb\nc\n"),
		},
		{
			name:  "filter_with_numeric_suffix",
			args:  []string{"--filter=cat > $FILE", "-d", "-l", "1"},
			stdin: []byte("x\ny\n"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			compareSplitOutputs(t, goBin, refBin, tc.args, tc.stdin)
		})
	}
}

// TestDiffExitCodes uses RunDiffTests for exit code and stderr comparison.
// R2.4: conflicting split options must produce an error and exit 1.
func TestDiffExitCodes(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsplit")
	if err != nil {
		t.Skipf("reference binary gsplit not in PATH: %v", err)
	}
	norm := []testutils.NormalizeFunc{stderrProgNormalizer}
	tests := []testutils.DiffTest{
		{
			Name:      "conflicting_lines_bytes",
			Args:      []string{"-l", "10", "-b", "100"},
			Stdin:     []byte("data\n"),
			ExitCode:  1,
			Normalize: norm,
		},
		{
			Name:      "conflicting_bytes_chunks",
			Args:      []string{"-b", "100", "-n", "3"},
			Stdin:     []byte("data\n"),
			ExitCode:  1,
			Normalize: norm,
		},
		{
			Name:      "conflicting_line_bytes_lines",
			Args:      []string{"-C", "100", "-l", "10"},
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
			t.Errorf("file %q: content mismatch\n  ref len=%d: %s\n  go  len=%d: %s",
				name, len(refData), abbreviate(refData), len(goData), abbreviate(goData))
		}
	}
}

// abbreviate returns a truncated representation of data for error messages.
func abbreviate(data []byte) string {
	s := string(data)
	if len(s) > 60 {
		s = s[:60] + "..."
	}
	return strings.ReplaceAll(s, "\n", "\\n")
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
