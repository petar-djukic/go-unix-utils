// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd068-csplit R4.1-R4.4.
// Covers integer patterns, /REGEXP/ patterns, %REGEXP% patterns,
// {REPEAT} and {*} suffixes, offset patterns, all flag combinations,
// and edge cases.
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

// binaryNameRe matches binary name prefixes in stderr including full paths.
var binaryNameRe = regexp.MustCompile(
	`(?:[\w/.-]*[/])?g?csplit:`)

// tryHelpRe matches "Try ... --help" hint lines.
var tryHelpRe = regexp.MustCompile(
	`(?m)^Try '.*' for more information\.\n`)

// stderrNormalizer normalizes csplit/gcsplit stderr differences:
// replaces binary name prefixes (including full paths) and removes
// "Try ... --help" lines.
func stderrNormalizer(data []byte) []byte {
	s := binaryNameRe.ReplaceAllString(string(data), "csplit:")
	s = tryHelpRe.ReplaceAllString(s, "")
	return []byte(s)
}

// TestDiff builds the csplit binary and compares output against gcsplit.
// R4.1-R4.4: uses testutils.BuildBinary, exec.LookPath("gcsplit"),
// and testutils.RunDiffTests for exit-code/stderr tests.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcsplit")
	if err != nil {
		t.Skip("reference binary gcsplit not in PATH")
	}

	t.Run("error_cases", func(t *testing.T) {
		runErrorTests(t, goBin, refBin)
	})
	t.Run("file_output", func(t *testing.T) {
		runFileOutputTests(t, goBin, refBin)
	})
}

// runErrorTests uses testutils.RunDiffTests for cases that produce
// stderr/exit code output without meaningful output files.
// R4.2: missing operand, missing pattern.
func runErrorTests(t *testing.T, goBin, refBin string) {
	t.Helper()
	norm := []testutils.NormalizeFunc{stderrNormalizer}
	tests := []testutils.DiffTest{
		{
			Name:      "missing_operand",
			Args:      []string{},
			Normalize: norm,
		},
		{
			Name:      "missing_pattern",
			Args:      []string{"-"},
			Stdin:     []byte("a\nb\n"),
			Normalize: norm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// fileTest describes a csplit invocation that produces output files.
type fileTest struct {
	name  string
	args  []string
	stdin string
}

// runFileOutputTests exercises csplit modes that produce output files,
// comparing files between separate reference and Go temp directories.
func runFileOutputTests(t *testing.T, goBin, refBin string) {
	t.Helper()
	tests := buildFileTests()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runAndCompare(t, goBin, refBin, tc.args, tc.stdin)
		})
	}
}

// buildFileTests returns all file-output test cases split across
// categories: basic patterns, repeats, flags, and edge cases.
func buildFileTests() []fileTest {
	var tests []fileTest
	tests = append(tests, basicPatternTests()...)
	tests = append(tests, repeatAndOffsetTests()...)
	tests = append(tests, flagTests()...)
	tests = append(tests, edgeCaseTests()...)
	return tests
}

// basicPatternTests covers R1.2 (/REGEXP/), R1.3 (%REGEXP%),
// R1.4 (INTEGER), and R4.1 (exit 0 on success).
func basicPatternTests() []fileTest {
	return []fileTest{
		// R1.2: regex split — match line starts next piece
		{
			name:  "regex_split",
			args:  []string{"-", "/c/"},
			stdin: "a\nb\nc\nd\n",
		},
		// R1.4: line number split
		{
			name:  "line_number_split",
			args:  []string{"-", "4", "7"},
			stdin: generateLines(10),
		},
		// R1.4: single line number
		{
			name:  "single_line_number",
			args:  []string{"-", "3"},
			stdin: generateLines(5),
		},
		// R1.2: multiple regex patterns
		{
			name:  "multiple_regex_patterns",
			args:  []string{"-", "/b/", "/d/"},
			stdin: "a\nb\nc\nd\ne\n",
		},
		// R1.3: skip pattern — no output file for skipped content
		{
			name:  "skip_pattern",
			args:  []string{"-", "%c%"},
			stdin: "a\nb\nc\nd\n",
		},
		// R1.3: skip then regex
		{
			name:  "skip_then_regex",
			args:  []string{"-", "%b%", "/d/"},
			stdin: "a\nb\nc\nd\ne\n",
		},
		// R1.2+R1.4: mixed line number and regex
		{
			name:  "mixed_line_and_regex",
			args:  []string{"-", "3", "/5/"},
			stdin: generateLines(7),
		},
	}
}

// repeatAndOffsetTests covers R2.1 ({N}), R2.2 ({*}),
// R2.3 (/REGEXP/+N, /REGEXP/-N).
func repeatAndOffsetTests() []fileTest {
	return []fileTest{
		// R2.1: {N} repeat count
		{
			name:  "repeat_count",
			args:  []string{"-", "/pattern/", "{1}"},
			stdin: "a\npattern\nb\npattern\nc\n",
		},
		// R2.2: {*} repeat to end
		{
			name:  "repeat_star",
			args:  []string{"-", "/pattern/", "{*}"},
			stdin: "line1\npattern\nline2\npattern\nline3\n",
		},
		// R2.2: {*} with many matches
		{
			name:  "repeat_star_many",
			args:  []string{"-", "/x/", "{*}"},
			stdin: "a\nx\nb\nx\nc\nx\nd\n",
		},
		// R2.3: positive offset /REGEXP/+N
		{
			name:  "regex_positive_offset",
			args:  []string{"-", "/b/+1"},
			stdin: "a\nb\nc\nd\ne\n",
		},
		// R2.3: negative offset /REGEXP/-1
		{
			name:  "regex_negative_offset",
			args:  []string{"-", "/c/-1"},
			stdin: "a\nb\nc\nd\ne\n",
		},
		// R2.3: zero offset (same as no offset)
		{
			name:  "regex_zero_offset",
			args:  []string{"-", "/c/+0"},
			stdin: "a\nb\nc\nd\ne\n",
		},
		// R2.1: {0} means no extra repeats (same as no modifier)
		{
			name:  "repeat_zero",
			args:  []string{"-", "/b/", "{0}"},
			stdin: "a\nb\nc\n",
		},
	}
}

// flagTests covers R3.1-R3.4: prefix, digits, elide-empty, silent,
// suppress-matched, and suffix-format.
func flagTests() []fileTest {
	return []fileTest{
		// R3.2: custom prefix -f
		{
			name:  "custom_prefix",
			args:  []string{"-f", "chunk", "-", "/c/"},
			stdin: "a\nb\nc\nd\n",
		},
		// R3.2: --prefix= form
		{
			name:  "prefix_long_form",
			args:  []string{"--prefix=part", "-", "/c/"},
			stdin: "a\nb\nc\nd\n",
		},
		// R3.3: custom digit width -n
		{
			name:  "custom_digits",
			args:  []string{"-n", "3", "-", "/c/"},
			stdin: "a\nb\nc\nd\n",
		},
		// R3.3: --digits= form
		{
			name:  "digits_long_form",
			args:  []string{"--digits=4", "-", "/c/"},
			stdin: "a\nb\nc\nd\n",
		},
		// R3.2+R3.3: prefix and digits combined
		{
			name:  "prefix_and_digits",
			args:  []string{"-f", "chunk", "-n", "3", "-", "/pattern/", "{*}"},
			stdin: "a\npattern\nb\npattern\nc\n",
		},
		// R3.4: elide empty files -z with line number causing empty piece
		{
			name:  "elide_empty",
			args:  []string{"-z", "-", "1"},
			stdin: generateLines(5),
		},
		// R3.4: --elide-empty-files
		{
			name:  "elide_empty_long",
			args:  []string{"--elide-empty-files", "-", "1"},
			stdin: generateLines(5),
		},
		// silent mode -s
		{
			name:  "silent_mode",
			args:  []string{"-s", "-", "/c/"},
			stdin: "a\nb\nc\nd\n",
		},
		// silent mode --quiet
		{
			name:  "quiet_mode",
			args:  []string{"--quiet", "-", "/c/"},
			stdin: "a\nb\nc\nd\n",
		},
		// --suppress-matched
		{
			name:  "suppress_matched",
			args:  []string{"--suppress-matched", "-", "/c/"},
			stdin: "a\nb\nc\nd\n",
		},
		// --suppress-matched with repeat
		{
			name:  "suppress_matched_repeat",
			args:  []string{"--suppress-matched", "-", "/x/", "{*}"},
			stdin: "a\nx\nb\nx\nc\n",
		},
		// suffix-format -b
		{
			name:  "suffix_format",
			args:  []string{"-b", "_%02d.txt", "-", "/c/"},
			stdin: "a\nb\nc\nd\n",
		},
		// --suffix-format=
		{
			name:  "suffix_format_long",
			args:  []string{"--suffix-format=%03d.dat", "-", "/c/"},
			stdin: "a\nb\nc\nd\n",
		},
	}
}

// edgeCaseTests covers R4.3 edge cases: match at line 1,
// consecutive patterns, special regex, no trailing newline.
func edgeCaseTests() []fileTest {
	return []fileTest{
		// Edge: pattern matches first line
		{
			name:  "match_at_line_1",
			args:  []string{"-", "/a/"},
			stdin: "a\nb\nc\n",
		},
		// Edge: line number 1 (split before first line)
		{
			name:  "line_number_1",
			args:  []string{"-", "1"},
			stdin: "a\nb\nc\n",
		},
		// Edge: single line input
		{
			name:  "single_line",
			args:  []string{"-", "/a/"},
			stdin: "a\n",
		},
		// Edge: no trailing newline
		{
			name:  "no_trailing_newline",
			args:  []string{"-", "/c/"},
			stdin: "a\nb\nc\nd",
		},
		// Edge: consecutive patterns matching same area
		{
			name:  "consecutive_close_patterns",
			args:  []string{"-", "/b/", "/c/"},
			stdin: "a\nb\nc\nd\n",
		},
		// Edge: regex with character class
		{
			name:  "regex_special_chars",
			args:  []string{"-", "/^line[0-9]/"},
			stdin: "header\nline1\nline2\n",
		},
		// Edge: large input with line number split
		{
			name:  "large_line_number",
			args:  []string{"-", "50", "100"},
			stdin: generateLines(150),
		},
		// Edge: repeat with line number
		{
			name:  "line_number_repeat",
			args:  []string{"-", "3", "{2}"},
			stdin: generateLines(12),
		},
		// Edge: elide-empty with regex creating empty pieces
		{
			name:  "elide_empty_regex",
			args:  []string{"-z", "-", "/^1/", "/^2/"},
			stdin: "1\n2\n3\n",
		},
		// Edge: regex with caret anchor
		{
			name:  "regex_caret_anchor",
			args:  []string{"-", "/^line/"},
			stdin: "header\nline one\nmore\nline two\n",
		},
		// Edge: regex dot matches any character
		{
			name:  "regex_dot_match",
			args:  []string{"-", "/x.z/"},
			stdin: "abc\nxyz\ndef\n",
		},
		// Edge: line split at last line
		{
			name:  "split_at_last_line",
			args:  []string{"-", "5"},
			stdin: generateLines(5),
		},
		// Edge: multiple line numbers ascending
		{
			name:  "multiple_line_numbers",
			args:  []string{"-", "2", "4", "6"},
			stdin: generateLines(8),
		},
		// Edge: skip then remaining content
		{
			name:  "skip_to_middle",
			args:  []string{"-", "%3%"},
			stdin: generateLines(6),
		},
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
// stdout, output files, stderr, and exit codes.
func runAndCompare(
	t *testing.T, goBin, refBin string, args []string, stdin string,
) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	refRes := runCsplit(t, refBin, args, stdin, refDir)
	goRes := runCsplit(t, goBin, args, stdin, goDir)

	compareResults(t, refRes, goRes)
	compareOutputFiles(t, refDir, goDir)
}

// csplitResult holds captured output of a csplit invocation.
type csplitResult struct {
	stdout   string
	stderr   string
	exitCode int
}

// runCsplit executes a csplit binary in the given directory.
func runCsplit(
	t *testing.T, bin string, args []string, stdin, dir string,
) csplitResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
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

	return csplitResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: exitCode,
	}
}

// compareResults checks stdout, stderr, and exit code match.
func compareResults(t *testing.T, ref, got csplitResult) {
	t.Helper()
	if ref.exitCode != got.exitCode {
		t.Errorf("exit code: ref=%d go=%d", ref.exitCode, got.exitCode)
	}
	if ref.stdout != got.stdout {
		t.Errorf("stdout diff:\n  ref: %q\n  go:  %q",
			ref.stdout, got.stdout)
	}
	normRef := string(stderrNormalizer([]byte(ref.stderr)))
	normGo := string(stderrNormalizer([]byte(got.stderr)))
	if normRef != normGo {
		t.Errorf("stderr diff:\n  ref: %q\n  go:  %q",
			normRef, normGo)
	}
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
