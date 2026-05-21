// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcsplit")
	if err != nil {
		t.Skip("reference binary not found")
	}
	tests := regexpSplitTests()
	tests = append(tests, lineNumberTests()...)
	tests = append(tests, skipPatternTests()...)
	tests = append(tests, fileInputTests(t)...)
	tests = append(tests, errorTests()...)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

var binaryNameRe = regexp.MustCompile(`\S*csplit:`)

func normalizeErr(b []byte) []byte {
	b = binaryNameRe.ReplaceAll(b, []byte("csplit:"))
	var out []byte
	for line := range bytes.SplitSeq(b, []byte("\n")) {
		if bytes.Contains(line, []byte("--help")) {
			continue
		}
		if len(out) > 0 {
			out = append(out, '\n')
		}
		out = append(out, line...)
	}
	return out
}

func regexpSplitTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "r1_2_regex_split_ac1",
			Args:  []string{"-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
			ExpectedFiles: map[string][]byte{
				"xx00": []byte("a\nb\n"),
				"xx01": []byte("c\nd\n"),
			},
		},
		{
			Name:  "r1_2_regex_first_line",
			Args:  []string{"-", "/a/"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			Name:  "r1_1_multiple_regex_patterns",
			Args:  []string{"-", "/b/", "/d/"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
			ExpectedFiles: map[string][]byte{
				"xx00": []byte("a\n"),
				"xx01": []byte("b\nc\n"),
				"xx02": []byte("d\ne\n"),
			},
		},
	}
}

func lineNumberTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "r1_4_line_number_ac2",
			Args:  []string{"-", "4", "7"},
			Stdin: seqBytes(1, 10),
			ExpectedFiles: map[string][]byte{
				"xx00": seqBytes(1, 3),
				"xx01": seqBytes(4, 6),
				"xx02": seqBytes(7, 10),
			},
		},
		{
			Name:  "r1_4_single_line_number",
			Args:  []string{"-", "3"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
			ExpectedFiles: map[string][]byte{
				"xx00": []byte("a\nb\n"),
				"xx01": []byte("c\nd\ne\n"),
			},
		},
	}
}

func skipPatternTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "r1_3_skip_then_split",
			Args:  []string{"-", "%c%", "/e/"},
			Stdin: []byte("a\nb\nc\nd\ne\nf\n"),
		},
		{
			Name:  "r1_3_skip_to_remainder",
			Args:  []string{"-", "%c%"},
			Stdin: []byte("a\nb\nc\nd\n"),
			ExpectedFiles: map[string][]byte{
				"xx00": []byte("c\nd\n"),
			},
		},
	}
}

func fileInputTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "input.txt", "alpha\nbeta\ngamma\ndelta\n")
	return []testutils.DiffTest{
		{
			Name:    "r1_2_file_input",
			Args:    []string{"input.txt", "/gamma/"},
			WorkDir: dir,
		},
	}
}

func errorTests() []testutils.DiffTest {
	norm := []testutils.NormalizeFunc{normalizeErr}
	return []testutils.DiffTest{
		{
			Name:      "r1_2_no_match_error",
			Args:      []string{"-", "/nomatch/"},
			Stdin:     []byte("a\nb\nc\n"),
			ExitCode:  1,
			Normalize: norm,
		},
		{
			Name:      "r1_3_skip_no_match",
			Args:      []string{"-", "%nomatch%"},
			Stdin:     []byte("a\nb\n"),
			ExitCode:  1,
			Normalize: norm,
		},
	}
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func seqBytes(from, to int) []byte {
	var buf bytes.Buffer
	for i := from; i <= to; i++ {
		fmt.Fprintf(&buf, "%d\n", i)
	}
	return buf.Bytes()
}
