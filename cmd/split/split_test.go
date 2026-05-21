// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsplit")
	if err != nil {
		t.Skip("reference binary not found")
	}
	tests := lineSplitTests()
	tests = append(tests, prefixAndStdinTests()...)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func lineSplitTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "default_1000_lines",
			Stdin: generateLines(1, 2500),
			ExpectedFiles: map[string][]byte{
				"xaa": generateLines(1, 1000),
				"xab": generateLines(1001, 2000),
				"xac": generateLines(2001, 2500),
			},
		},
		{
			Name:  "custom_line_count",
			Args:  []string{"-l", "3"},
			Stdin: generateLines(1, 7),
			ExpectedFiles: map[string][]byte{
				"xaa": generateLines(1, 3),
				"xab": generateLines(4, 6),
				"xac": generateLines(7, 7),
			},
		},
		{
			Name:  "lines_long_equals",
			Args:  []string{"--lines=4"},
			Stdin: generateLines(1, 10),
			ExpectedFiles: map[string][]byte{
				"xaa": generateLines(1, 4),
				"xab": generateLines(5, 8),
				"xac": generateLines(9, 10),
			},
		},
	}
}

func prefixAndStdinTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "custom_prefix",
			Args:  []string{"-l", "3", "-", "chunk_"},
			Stdin: generateLines(1, 5),
			ExpectedFiles: map[string][]byte{
				"chunk_aa": generateLines(1, 3),
				"chunk_ab": generateLines(4, 5),
			},
		},
		{
			Name:  "stdin_explicit_dash",
			Args:  []string{"-l", "2", "-"},
			Stdin: generateLines(1, 3),
			ExpectedFiles: map[string][]byte{
				"xaa": generateLines(1, 2),
				"xab": generateLines(3, 3),
			},
		},
	}
}

func generateLines(from, to int) []byte {
	var buf bytes.Buffer
	for i := from; i <= to; i++ {
		fmt.Fprintf(&buf, "%d\n", i)
	}
	return buf.Bytes()
}
