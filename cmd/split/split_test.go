// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/split.
// Covers prd067-split R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
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

// clearOutput is a normalizer that clears output for error-only tests.
// Used for R2.4 conflict tests where stderr messages differ between
// split and gsplit but exit codes must match.
func clearOutput(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsplit")
	if err != nil {
		t.Skip("reference binary gsplit not in PATH")
	}
	tests := append(buildR1Tests(), buildR2Tests()...)
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

// buildR2Tests returns differential test cases for R2.1-R2.4.
func buildR2Tests() []testutils.DiffTest {
	return []testutils.DiffTest{
		byteSplitTest(),
		byteSplitSuffixTest(),
		lineBytesSplitTest(),
		lineBytesLongLineTest(),
		chunkByBytesTest(),
		chunkByLinesTest(),
		chunkRoundRobinTest(),
		conflictingModesTest(),
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

// byteSplitTest verifies R2.1: -b splits by byte count.
func byteSplitTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "byte_split",
		Args:     []string{"-b", "4"},
		Stdin:    bytes.Repeat([]byte{'a'}, 10),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa": bytes.Repeat([]byte{'a'}, 4),
			"xab": bytes.Repeat([]byte{'a'}, 4),
			"xac": bytes.Repeat([]byte{'a'}, 2),
		},
	}
}

// byteSplitSuffixTest verifies R2.1: -b with K suffix (1024 bytes).
func byteSplitSuffixTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "byte_split_suffix_K",
		Args:     []string{"-b", "1K"},
		Stdin:    bytes.Repeat([]byte{'x'}, 2048),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa": bytes.Repeat([]byte{'x'}, 1024),
			"xab": bytes.Repeat([]byte{'x'}, 1024),
		},
	}
}

// lineBytesSplitTest verifies R2.2: -C splits at line boundaries.
func lineBytesSplitTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "line_bytes_split",
		Args:     []string{"-C", "6"},
		Stdin:    []byte("1\n2\n3\n4\n5\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa": []byte("1\n2\n3\n"),
			"xab": []byte("4\n5\n"),
		},
	}
}

// lineBytesLongLineTest verifies R2.2: lines longer than N get split.
func lineBytesLongLineTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "line_bytes_long_line",
		Args:     []string{"-C", "5"},
		Stdin:    []byte("abcdefgh\n"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa": []byte("abcde"),
			"xab": []byte("fgh\n"),
		},
	}
}

// chunkByBytesTest verifies R2.3: -n N splits into N equal byte chunks.
func chunkByBytesTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "chunk_by_bytes",
		Args:     []string{"-n", "3"},
		Stdin:    []byte("123456789"),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa": []byte("123"),
			"xab": []byte("456"),
			"xac": []byte("789"),
		},
	}
}

// chunkByLinesTest verifies R2.3: -n l/N splits into N line-balanced chunks.
func chunkByLinesTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "chunk_by_lines",
		Args:     []string{"-n", "l/3"},
		Stdin:    generateLines(1, 9),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa": generateLines(1, 3),
			"xab": generateLines(4, 3),
			"xac": generateLines(7, 3),
		},
	}
}

// chunkRoundRobinTest verifies R2.3: -n r/N round-robin distributes lines.
func chunkRoundRobinTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "chunk_round_robin",
		Args:     []string{"-n", "r/3"},
		Stdin:    generateLines(1, 6),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa": []byte("1\n4\n"),
			"xab": []byte("2\n5\n"),
			"xac": []byte("3\n6\n"),
		},
	}
}

// conflictingModesTest verifies R2.4: conflicting split modes produce exit 1.
func conflictingModesTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:      "conflicting_modes_error",
		Args:      []string{"-l", "5", "-b", "10"},
		Stdin:     []byte("test\n"),
		ExitCode:  1,
		Normalize: []testutils.NormalizeFunc{clearOutput},
	}
}
