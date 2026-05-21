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
	tests = append(tests, repeatTests()...)
	tests = append(tests, offsetTests()...)
	tests = append(tests, prefixTests()...)
	tests = append(tests, digitsTests()...)
	tests = append(tests, elideEmptyTests()...)
	tests = append(tests, combinedOptionTests()...)
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
			ExpectedFiles: map[string][]byte{
				"xx00": []byte(""),
				"xx01": []byte("a\nb\nc\n"),
			},
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
			ExpectedFiles: map[string][]byte{
				"xx00": []byte("c\nd\n"),
				"xx01": []byte("e\nf\n"),
			},
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

func repeatTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "r2_1_repeat_n",
			Args:  []string{"-", "/^---$/", "{1}"},
			Stdin: []byte("a\n---\nb\n---\nc\n---\nd\n"),
			ExpectedFiles: map[string][]byte{
				"xx00": []byte("a\n"),
				"xx01": []byte("---\nb\n"),
				"xx02": []byte("---\nc\n---\nd\n"),
			},
		},
		{
			Name:  "r2_2_repeat_star",
			Args:  []string{"-", "/^---$/", "{*}"},
			Stdin: []byte("a\n---\nb\n---\nc\n---\nd\n"),
			ExpectedFiles: map[string][]byte{
				"xx00": []byte("a\n"),
				"xx01": []byte("---\nb\n"),
				"xx02": []byte("---\nc\n"),
				"xx03": []byte("---\nd\n"),
			},
		},
		{
			Name:  "r2_1_repeat_line_number",
			Args:  []string{"-", "3", "{1}"},
			Stdin: seqBytes(1, 9),
			ExpectedFiles: map[string][]byte{
				"xx00": seqBytes(1, 2),
				"xx01": seqBytes(3, 5),
				"xx02": seqBytes(6, 9),
			},
		},
	}
}

func offsetTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "r2_3_offset_plus",
			Args:  []string{"-", "/c/+1"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
			ExpectedFiles: map[string][]byte{
				"xx00": []byte("a\nb\nc\n"),
				"xx01": []byte("d\ne\n"),
			},
		},
		{
			Name:  "r2_3_offset_minus",
			Args:  []string{"-", "/c/-1"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
			ExpectedFiles: map[string][]byte{
				"xx00": []byte("a\n"),
				"xx01": []byte("b\nc\nd\ne\n"),
			},
		},
		{
			Name:  "r2_3_offset_plus_two",
			Args:  []string{"-", "/b/+2"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
			ExpectedFiles: map[string][]byte{
				"xx00": []byte("a\nb\nc\n"),
				"xx01": []byte("d\ne\n"),
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

func prefixTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "r3_2_custom_prefix_short",
			Args:  []string{"-f", "chunk", "-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			Name:  "r3_2_custom_prefix_long",
			Args:  []string{"--prefix=part", "-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			Name:  "r3_2_custom_prefix_attached",
			Args:  []string{"-fout", "-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
	}
}

func digitsTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "r3_3_custom_digits_short",
			Args:  []string{"-n", "3", "-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		{
			Name:  "r3_3_custom_digits_long",
			Args:  []string{"--digits=4", "-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
	}
}

func elideEmptyTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "r3_4_elide_empty_short",
			Args:  []string{"-z", "-", "/a/"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			Name:  "r3_4_elide_empty_long",
			Args:  []string{"--elide-empty-files", "-", "/a/"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			Name:  "r3_4_elide_no_effect",
			Args:  []string{"-z", "-", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
		},
	}
}

func combinedOptionTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "r3_ac3_prefix_digits_repeat",
			Args:  []string{"-f", "chunk", "-n", "3", "-", "/^---$/", "{*}"},
			Stdin: []byte("a\n---\nb\n---\nc\n---\nd\n"),
		},
		{
			Name:  "r3_prefix_digits_elide",
			Args:  []string{"-f", "out", "-n", "3", "-z", "-", "/a/", "/c/"},
			Stdin: []byte("a\nb\nc\nd\n"),
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
		{
			Name:      "r2_4_repeat_no_match",
			Args:      []string{"-", "/x/", "{2}"},
			Stdin:     []byte("a\nx\nb\nc\n"),
			ExitCode:  1,
			Normalize: norm,
		},
		{
			Name:     "r4_2_invalid_regex",
			Args:     []string{"-", "/[/"},
			Stdin:    []byte("a\nb\n"),
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{func(b []byte) []byte {
				if len(b) == 0 {
					return b
				}
				return []byte("<error>")
			}},
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
