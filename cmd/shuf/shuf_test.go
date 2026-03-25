// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/shuf against GNU gshuf.
// Covers prd064-shuf R4.1-R4.4 (differential testing).
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// sortLinesNorm sorts newline-delimited output lines for
// order-independent content comparison.
// R4.3: verify output line set, not order.
func sortLinesNorm() testutils.NormalizeFunc {
	return func(b []byte) []byte {
		if len(b) == 0 {
			return b
		}
		s := strings.TrimSuffix(string(b), "\n")
		if s == "" {
			return []byte("\n")
		}
		lines := strings.Split(s, "\n")
		sort.Strings(lines)
		return []byte(strings.Join(lines, "\n") + "\n")
	}
}

// lineCountNorm replaces stdout with the line count string,
// for verifying output count without comparing content.
// R4.3/D4: verify line count for -n tests.
func lineCountNorm() testutils.NormalizeFunc {
	return func(b []byte) []byte {
		if len(b) == 0 {
			return []byte("0\n")
		}
		s := strings.TrimSuffix(string(b), "\n")
		if s == "" {
			return []byte("0\n")
		}
		n := len(strings.Split(s, "\n"))
		return fmt.Appendf(nil, "%d\n", n)
	}
}

// sortNulNorm sorts NUL-delimited output entries for
// order-independent comparison of -z output.
func sortNulNorm() testutils.NormalizeFunc {
	return func(b []byte) []byte {
		if len(b) == 0 {
			return b
		}
		b = bytes.TrimSuffix(b, []byte{0})
		if len(b) == 0 {
			return []byte{0}
		}
		parts := strings.Split(string(b), "\x00")
		sort.Strings(parts)
		return []byte(strings.Join(parts, "\x00") + "\x00")
	}
}

// stderrNorm normalizes error messages between GNU gshuf and Go shuf.
func stderrNorm() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?shuf|gshuf`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("shuf"))
		b = tryHelp.ReplaceAll(b, nil)
		return b
	}
}

// discardStdout replaces stdout with empty bytes so only exit code
// and stderr are compared (used for -o output-file tests).
func discardStdout() testutils.NormalizeFunc {
	return func(b []byte) []byte {
		return nil
	}
}

// writeFixture creates a file in dir with the given content.
func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshuf")
	if err != nil {
		t.Skipf("reference binary gshuf not in PATH: %v", err)
	}

	tests := buildBasicShufTests(t)
	tests = append(tests, buildFlagShufTests(t)...)
	tests = append(tests, buildErrorShufTests()...)
	tests = append(tests, buildEdgeShufTests()...)

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildBasicShufTests returns tests for R4.1: basic shuffling modes.
func buildBasicShufTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	sortNorm := sortLinesNorm()

	fileDir := t.TempDir()
	writeFixture(t, fileDir, "input.txt", "x\ny\nz\n")

	return []testutils.DiffTest{
		// R4.1/R1.2: shuffle stdin lines, verify same set via sort.
		{
			Name:      "stdin_shuffle",
			Stdin:     []byte("a\nb\nc\n"),
			Normalize: []testutils.NormalizeFunc{sortNorm},
		},
		// R4.1/R1.1: shuffle file lines, verify same set via sort.
		{
			Name:      "file_shuffle",
			Args:      []string{"input.txt"},
			WorkDir:   fileDir,
			Normalize: []testutils.NormalizeFunc{sortNorm},
		},
		// R4.1/R2.1: shuffle -i range, verify same integer set.
		{
			Name:      "range_shuffle",
			Args:      []string{"-i", "1-5"},
			Normalize: []testutils.NormalizeFunc{sortNorm},
		},
		// R4.1/R1.2: "-" means stdin.
		{
			Name:      "stdin_dash_arg",
			Args:      []string{"-"},
			Stdin:     []byte("alpha\nbeta\ngamma\n"),
			Normalize: []testutils.NormalizeFunc{sortNorm},
		},
	}
}

// buildFlagShufTests returns tests for R4.2: flag combinations.
func buildFlagShufTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	countNorm := lineCountNorm()
	sortNorm := sortLinesNorm()
	nulSort := sortNulNorm()

	outDir := t.TempDir()

	return []testutils.DiffTest{
		// R4.2/R2.2: -n limits output count.
		{
			Name:      "head_count_range",
			Args:      []string{"-i", "1-10", "-n", "3"},
			Normalize: []testutils.NormalizeFunc{countNorm},
		},
		// R4.2/R2.2: -n with stdin.
		{
			Name:      "head_count_stdin",
			Args:      []string{"-n", "2"},
			Stdin:     []byte("a\nb\nc\nd\ne\n"),
			Normalize: []testutils.NormalizeFunc{countNorm},
		},
		// R4.2/R2.4: -o writes output to file, stdout is empty.
		{
			Name:      "output_file",
			Args:      []string{"-i", "1-5", "-o", "out.txt"},
			WorkDir:   outDir,
			Normalize: []testutils.NormalizeFunc{discardStdout()},
		},
		// R4.2/R3.2: -z uses NUL delimiter, verify same set.
		{
			Name:      "zero_terminated_stdin",
			Args:      []string{"-z"},
			Stdin:     []byte("a\x00b\x00c\x00"),
			Normalize: []testutils.NormalizeFunc{nulSort},
		},
		// R4.2/R2.3: -r -n for repeat mode with count.
		{
			Name:      "repeat_with_count",
			Args:      []string{"-r", "-n", "10", "-i", "1-5"},
			Normalize: []testutils.NormalizeFunc{countNorm},
		},
		// R4.2/R2.2+R2.1: --head-count= long form.
		{
			Name:      "head_count_long_form",
			Args:      []string{"--head-count=2", "-i", "1-5"},
			Normalize: []testutils.NormalizeFunc{countNorm},
		},
		// R4.2/R2.1: --input-range= long form.
		{
			Name:      "input_range_long_form",
			Args:      []string{"--input-range=1-3"},
			Normalize: []testutils.NormalizeFunc{sortNorm},
		},
		// R4.2/R3.2: -z with -i range.
		{
			Name:      "zero_terminated_range",
			Args:      []string{"-z", "-i", "1-3"},
			Normalize: []testutils.NormalizeFunc{nulSort},
		},
	}
}

// buildErrorShufTests returns tests for R4.3: error conditions.
func buildErrorShufTests() []testutils.DiffTest {
	errNorm := stderrNorm()
	return []testutils.DiffTest{
		// R4.3/R4.2: invalid range (LO > HI).
		{
			Name:      "error_invalid_range_reversed",
			Args:      []string{"-i", "5-1"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3/R4.2: -i combined with file operand.
		{
			Name:      "error_range_with_file",
			Args:      []string{"-i", "1-5", "/dev/null"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3/R4.2: invalid range format (no dash).
			{
			Name:      "error_invalid_range_format",
			Args:      []string{"-i", "abc"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3/R4.2: invalid -n value.
		{
			Name:      "error_invalid_head_count",
			Args:      []string{"-n", "abc"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
}

// buildEdgeShufTests returns tests for R4.4: edge cases.
func buildEdgeShufTests() []testutils.DiffTest {
	sortNorm := sortLinesNorm()
	countNorm := lineCountNorm()
	return []testutils.DiffTest{
		// R4.4/R3.4: empty input produces no output.
		{
			Name:  "empty_stdin",
			Stdin: []byte(""),
		},
		// R4.4: single-line input.
		{
			Name:      "single_line_stdin",
			Stdin:     []byte("only\n"),
			Normalize: []testutils.NormalizeFunc{sortNorm},
		},
		// R4.4: -n 0 produces no output.
		{
			Name: "head_count_zero",
			Args: []string{"-i", "1-5", "-n", "0"},
		},
		// R4.4: -i with equal LO and HI produces one line.
		{
			Name:      "range_single_value",
			Args:      []string{"-i", "5-5"},
			Normalize: []testutils.NormalizeFunc{sortNorm},
		},
		// R4.4/R1.4: last line without trailing newline.
		{
			Name:      "no_trailing_newline",
			Stdin:     []byte("a\nb\nc"),
			Normalize: []testutils.NormalizeFunc{sortNorm},
		},
		// R4.4: -r with -n 1 from single value range.
		{
			Name:      "repeat_single_value",
			Args:      []string{"-r", "-n", "1", "-i", "5-5"},
			Normalize: []testutils.NormalizeFunc{countNorm},
		},
		// R4.4: large range shuffled, verify count.
		{
			Name:      "large_range_count",
			Args:      []string{"-i", "1-100"},
			Normalize: []testutils.NormalizeFunc{countNorm},
		},
		// R4.4: -n larger than input, outputs all lines.
		{
			Name:      "head_count_exceeds_input",
			Args:      []string{"-n", "10", "-i", "1-3"},
			Normalize: []testutils.NormalizeFunc{sortNorm},
		},
	}
}
