// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/split.
// Covers prd067-split R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// generateLines produces numbered lines from start to start+count-1.
func generateLines(start, count int) []byte {
	var buf bytes.Buffer
	for i := start; i < start+count; i++ {
		fmt.Fprintf(&buf, "%d\n", i)
	}
	return buf.Bytes()
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsplit")
	if err != nil {
		t.Skip("reference binary gsplit not in PATH")
	}
	tests := buildR1Tests()
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildR1Tests returns differential test cases for R1.1-R1.4.
func buildR1Tests() []testutils.DiffTest {
	return []testutils.DiffTest{
		defaultSplitTest(),
		customLineCountTest(),
		customPrefixTest(),
		stdinViaDashTest(),
		singleFileOutputTest(),
	}
}

// defaultSplitTest verifies R1.1: default 1000-line split with xaa/xab/xac naming.
func defaultSplitTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "default_1000_lines",
		Stdin:    generateLines(1, 2001),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa": generateLines(1, 1000),
			"xab": generateLines(1001, 1000),
			"xac": generateLines(2001, 1),
		},
	}
}

// customLineCountTest verifies R1.3: -l N splits into N-line pieces.
func customLineCountTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "custom_line_count",
		Args:     []string{"-l", "3"},
		Stdin:    generateLines(1, 7),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa": generateLines(1, 3),
			"xab": generateLines(4, 3),
			"xac": generateLines(7, 1),
		},
	}
}

// customPrefixTest verifies R1.2: PREFIX argument replaces default "x".
func customPrefixTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "custom_prefix",
		Args:     []string{"-l", "2", "-", "chunk_"},
		Stdin:    generateLines(1, 5),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"chunk_aa": generateLines(1, 2),
			"chunk_ab": generateLines(3, 2),
			"chunk_ac": generateLines(5, 1),
		},
	}
}

// stdinViaDashTest verifies R1.4: "-" as FILE reads from stdin.
func stdinViaDashTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "stdin_via_dash",
		Args:     []string{"-l", "2", "-"},
		Stdin:    generateLines(1, 4),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa": generateLines(1, 2),
			"xab": generateLines(3, 2),
		},
	}
}

// singleFileOutputTest verifies R1.1/R1.3: input smaller than chunk size
// produces a single output file.
func singleFileOutputTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "single_file_output",
		Args:     []string{"-l", "10"},
		Stdin:    generateLines(1, 3),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa": generateLines(1, 3),
		},
	}
}
