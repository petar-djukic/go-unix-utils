// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/csplit.
// Covers prd068-csplit R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// generateSeq produces numbered lines from start to end (inclusive).
func generateSeq(start, end int) []byte {
	var buf bytes.Buffer
	for i := start; i <= end; i++ {
		fmt.Fprintf(&buf, "%d\n", i)
	}
	return buf.Bytes()
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcsplit")
	if err != nil {
		t.Skip("reference binary gcsplit not in PATH")
	}
	tests := buildR1Tests()
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildR1Tests returns differential test cases for R1.1-R1.4.
func buildR1Tests() []testutils.DiffTest {
	return []testutils.DiffTest{
		regexSplitTest(),
		lineNumberSplitTest(),
		skipPatternTest(),
		mixedPatternsTest(),
		skipOnlyTest(),
	}
}

// regexSplitTest verifies R1.2: /REGEXP/ splits at the matching line.
func regexSplitTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "regex_split",
		Args:     []string{"-", "/c/"},
		Stdin:    []byte("a\nb\nc\nd\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xx00": []byte("a\nb\n"),
			"xx01": []byte("c\nd\n"),
		},
	}
}

// lineNumberSplitTest verifies R1.4: INTEGER splits at the given line number.
func lineNumberSplitTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "line_number_split",
		Args:     []string{"-", "4", "7"},
		Stdin:    generateSeq(1, 10),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xx00": generateSeq(1, 3),
			"xx01": generateSeq(4, 6),
			"xx02": generateSeq(7, 10),
		},
	}
}

// skipPatternTest verifies R1.3: %REGEXP% skips without creating an output file.
func skipPatternTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "skip_pattern",
		Args:     []string{"-", "%c%", "/e/"},
		Stdin:    []byte("a\nb\nc\nd\ne\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xx00": []byte("c\nd\n"),
			"xx01": []byte("e\n"),
		},
	}
}

// mixedPatternsTest verifies R1.1: multiple pattern types applied in order.
func mixedPatternsTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "mixed_patterns",
		Args:     []string{"-", "3", "/e/"},
		Stdin:    []byte("a\nb\nc\nd\ne\nf\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xx00": []byte("a\nb\n"),
			"xx01": []byte("c\nd\n"),
			"xx02": []byte("e\nf\n"),
		},
	}
}

// skipOnlyTest verifies R1.3: skip pattern followed by remaining content.
func skipOnlyTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "skip_only",
		Args:     []string{"-", "%c%"},
		Stdin:    []byte("a\nb\nc\nd\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xx00": []byte("c\nd\n"),
		},
	}
}
