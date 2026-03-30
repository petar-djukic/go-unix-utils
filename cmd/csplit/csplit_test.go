// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/csplit.
// Covers prd068-csplit R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4.
package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrBinaryRe matches the binary name prefix in stderr error messages.
var stderrBinaryRe = regexp.MustCompile(`\S*g?csplit: `)

// binaryNameNormalizer normalizes binary path prefixes in stderr messages
// so that /opt/homebrew/bin/gcsplit and csplit compare equal.
func binaryNameNormalizer() testutils.NormalizeFunc {
	return func(data []byte) []byte {
		return stderrBinaryRe.ReplaceAll(data, []byte("csplit: "))
	}
}

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
	tests := append(buildR1Tests(), buildR2Tests()...)
	tests = append(tests, buildR3Tests()...)
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

// buildR2Tests returns differential test cases for R2.1-R2.4.
func buildR2Tests() []testutils.DiffTest {
	return []testutils.DiffTest{
		repeatCountNTest(),
		repeatStarTest(),
		offsetPlusTest(),
		offsetMinusTest(),
		noMatchErrorTest(),
		repeatNOverflowTest(),
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

// repeatCountNTest verifies R2.1: {N} repeats the pattern N additional times.
func repeatCountNTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "repeat_count_n",
		Args:     []string{"-", "/---/", "{2}"},
		Stdin:    []byte("a\n---\nb\n---\nc\n---\nd\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xx00": []byte("a\n"),
			"xx01": []byte("---\nb\n"),
			"xx02": []byte("---\nc\n"),
			"xx03": []byte("---\nd\n"),
		},
	}
}

// repeatStarTest verifies R2.2: {*} repeats the pattern until end of input.
func repeatStarTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "repeat_star",
		Args:     []string{"-", "/---/", "{*}"},
		Stdin:    []byte("a\n---\nb\n---\nc\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xx00": []byte("a\n"),
			"xx01": []byte("---\nb\n"),
			"xx02": []byte("---\nc\n"),
		},
	}
}

// offsetPlusTest verifies R2.3: /REGEXP/+N splits N lines after the match.
func offsetPlusTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "offset_plus",
		Args:     []string{"-", "/c/+1"},
		Stdin:    []byte("a\nb\nc\nd\ne\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xx00": []byte("a\nb\nc\n"),
			"xx01": []byte("d\ne\n"),
		},
	}
}

// offsetMinusTest verifies R2.3: /REGEXP/-N splits N lines before the match.
func offsetMinusTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "offset_minus",
		Args:     []string{"-", "/c/-1"},
		Stdin:    []byte("a\nb\nc\nd\ne\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xx00": []byte("a\n"),
			"xx01": []byte("b\nc\nd\ne\n"),
		},
	}
}

// noMatchErrorTest verifies R2.4: error on stderr when pattern does not match.
func noMatchErrorTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:      "no_match_error",
		Args:      []string{"-", "/nomatch/"},
		Stdin:     []byte("a\nb\nc\n"),
		ExitCode:  1,
		Normalize: []testutils.NormalizeFunc{binaryNameNormalizer()},
	}
}

// repeatNOverflowTest verifies R2.1+R2.4: {N} fails when not enough matches.
func repeatNOverflowTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:      "repeat_n_overflow",
		Args:      []string{"-", "/---/", "{3}"},
		Stdin:     []byte("a\n---\nb\n"),
		ExitCode:  1,
		Normalize: []testutils.NormalizeFunc{binaryNameNormalizer()},
	}
}

// buildR3Tests returns differential test cases for R3.1-R3.4.
func buildR3Tests() []testutils.DiffTest {
	return []testutils.DiffTest{
		defaultPrefixTest(),
		customPrefixTest(),
		customDigitsTest(),
		prefixAndDigitsTest(),
		elideEmptyTest(),
	}
}

// defaultPrefixTest verifies R3.1: output files named xx00, xx01, etc. by default.
func defaultPrefixTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "default_prefix",
		Args:     []string{"-", "3"},
		Stdin:    generateSeq(1, 5),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xx00": generateSeq(1, 2),
			"xx01": generateSeq(3, 5),
		},
	}
}

// customPrefixTest verifies R3.2: -f PREFIX uses PREFIX instead of 'xx'.
func customPrefixTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "custom_prefix",
		Args:     []string{"-f", "chunk", "-", "/c/"},
		Stdin:    []byte("a\nb\nc\nd\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"chunk00": []byte("a\nb\n"),
			"chunk01": []byte("c\nd\n"),
		},
	}
}

// customDigitsTest verifies R3.3: -n DIGITS uses DIGITS-wide numeric suffixes.
func customDigitsTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "custom_digits",
		Args:     []string{"-n", "4", "-", "/c/"},
		Stdin:    []byte("a\nb\nc\nd\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xx0000": []byte("a\nb\n"),
			"xx0001": []byte("c\nd\n"),
		},
	}
}

// prefixAndDigitsTest verifies R3.2+R3.3 combined: -f and -n together.
func prefixAndDigitsTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "prefix_and_digits",
		Args:     []string{"-f", "part", "-n", "3", "-", "/---/", "{*}"},
		Stdin:    []byte("a\n---\nb\n---\nc\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"part000": []byte("a\n"),
			"part001": []byte("---\nb\n"),
			"part002": []byte("---\nc\n"),
		},
	}
}

// elideEmptyTest verifies R3.4: -z suppresses creation of empty output files.
func elideEmptyTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "elide_empty",
		Args:     []string{"-z", "-", "/a/"},
		Stdin:    []byte("a\nb\nc\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xx00": []byte("a\nb\nc\n"),
		},
	}
}
