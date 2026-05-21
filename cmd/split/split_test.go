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
	tests = append(tests, byteSplitTests()...)
	tests = append(tests, lineByteTests()...)
	tests = append(tests, chunkTests()...)
	tests = append(tests, conflictTests()...)
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

func byteSplitTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "bytes_exact",
			Args:  []string{"-b", "10"},
			Stdin: bytes.Repeat([]byte("a"), 30),
			ExpectedFiles: map[string][]byte{
				"xaa": bytes.Repeat([]byte("a"), 10),
				"xab": bytes.Repeat([]byte("a"), 10),
				"xac": bytes.Repeat([]byte("a"), 10),
			},
		},
		{
			Name:  "bytes_remainder",
			Args:  []string{"--bytes=7"},
			Stdin: bytes.Repeat([]byte("b"), 20),
			ExpectedFiles: map[string][]byte{
				"xaa": bytes.Repeat([]byte("b"), 7),
				"xab": bytes.Repeat([]byte("b"), 7),
				"xac": bytes.Repeat([]byte("b"), 6),
			},
		},
	}
}

func lineByteTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "line_bytes_basic",
			Args:  []string{"-C", "15"},
			Stdin: []byte("hello\nworld\nfoo\nbar\nbaz\n"),
			ExpectedFiles: map[string][]byte{
				"xaa": []byte("hello\nworld\n"),
				"xab": []byte("foo\nbar\nbaz\n"),
			},
		},
		{
			Name:  "line_bytes_long_line",
			Args:  []string{"--line-bytes=5"},
			Stdin: []byte("abcdefghij\nxy\n"),
			ExpectedFiles: map[string][]byte{
				"xaa": []byte("abcde"),
				"xab": []byte("fghij"),
				"xac": []byte("\nxy\n"),
			},
		},
	}
}

func chunkTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "chunk_bytes",
			Args:  []string{"-n", "3"},
			Stdin: []byte("abcdefghi"),
			ExpectedFiles: map[string][]byte{
				"xaa": []byte("abc"),
				"xab": []byte("def"),
				"xac": []byte("ghi"),
			},
		},
		{
			Name:  "chunk_lines",
			Args:  []string{"-n", "l/3"},
			Stdin: generateLines(1, 9),
			ExpectedFiles: map[string][]byte{
				"xaa": generateLines(1, 3),
				"xab": generateLines(4, 6),
				"xac": generateLines(7, 9),
			},
		},
		{
			Name:  "chunk_round_robin",
			Args:  []string{"-n", "r/3"},
			Stdin: generateLines(1, 6),
			ExpectedFiles: map[string][]byte{
				"xaa": []byte("1\n4\n"),
				"xab": []byte("2\n5\n"),
				"xac": []byte("3\n6\n"),
			},
		},
	}
}

func conflictTests() []testutils.DiffTest {
	norm := []testutils.NormalizeFunc{normalizeCmdErr}
	return []testutils.DiffTest{
		{
			Name:      "conflict_lines_bytes",
			Args:      []string{"-l", "5", "-b", "100"},
			Stdin:     []byte("test\n"),
			ExitCode:  1,
			Normalize: norm,
		},
		{
			Name:      "conflict_bytes_number",
			Args:      []string{"-b", "100", "-n", "3"},
			Stdin:     []byte("test\n"),
			ExitCode:  1,
			Normalize: norm,
		},
	}
}

func normalizeCmdErr(b []byte) []byte {
	var out []byte
	for _, line := range bytes.Split(b, []byte("\n")) {
		if len(line) == 0 || bytes.HasPrefix(line, []byte("Try ")) {
			continue
		}
		if i := bytes.Index(line, []byte(": ")); i >= 0 {
			line = line[i+2:]
		}
		out = append(out, line...)
		out = append(out, '\n')
	}
	return out
}

func generateLines(from, to int) []byte {
	var buf bytes.Buffer
	for i := from; i <= to; i++ {
		fmt.Fprintf(&buf, "%d\n", i)
	}
	return buf.Bytes()
}
