// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/split.
// Covers prd067-split R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4.
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
	tests = append(tests, buildR3Tests()...)
	tests = append(tests, buildR4Tests()...)
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

// buildR3Tests returns differential test cases for R3.1-R3.4.
func buildR3Tests() []testutils.DiffTest {
	return []testutils.DiffTest{
		suffixLengthTest(),
		numericSuffixShortTest(),
		numericSuffixLongTest(),
		additionalSuffixTest(),
		numericWithSuffixLenTest(),
		filterCommandTest(),
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

// suffixLengthTest verifies R3.1: -a N uses suffixes of length N.
func suffixLengthTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "suffix_length_3",
		Args:     []string{"-l", "2", "-a", "3"},
		Stdin:    generateLines(1, 5),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaaa": generateLines(1, 2),
			"xaab": generateLines(3, 2),
			"xaac": generateLines(5, 1),
		},
	}
}

// numericSuffixShortTest verifies R3.2: -d uses numeric suffixes.
func numericSuffixShortTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "numeric_suffix_short",
		Args:     []string{"-l", "2", "-d"},
		Stdin:    generateLines(1, 5),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"x00": generateLines(1, 2),
			"x01": generateLines(3, 2),
			"x02": generateLines(5, 1),
		},
	}
}

// numericSuffixLongTest verifies R3.2: --numeric-suffixes uses numeric suffixes.
func numericSuffixLongTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "numeric_suffix_long",
		Args:     []string{"-l", "2", "--numeric-suffixes"},
		Stdin:    generateLines(1, 4),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"x00": generateLines(1, 2),
			"x01": generateLines(3, 2),
		},
	}
}

// additionalSuffixTest verifies R3.3: --additional-suffix appends to filenames.
func additionalSuffixTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "additional_suffix",
		Args:     []string{"-l", "2", "--additional-suffix=.txt"},
		Stdin:    generateLines(1, 5),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa.txt": generateLines(1, 2),
			"xab.txt": generateLines(3, 2),
			"xac.txt": generateLines(5, 1),
		},
	}
}

// numericWithSuffixLenTest verifies R3.1+R3.2 combined: -d -a 3.
func numericWithSuffixLenTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "numeric_suffix_length_3",
		Args:     []string{"-l", "2", "-d", "-a", "3"},
		Stdin:    generateLines(1, 5),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"x000": generateLines(1, 2),
			"x001": generateLines(3, 2),
			"x002": generateLines(5, 1),
		},
	}
}

// filterCommandTest verifies R3.4: --filter pipes output through a command.
func filterCommandTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "filter_command",
		Args:     []string{"-l", "2", "--filter=cat > $FILE"},
		Stdin:    generateLines(1, 4),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa": generateLines(1, 2),
			"xab": generateLines(3, 2),
		},
	}
}

// buildR4Tests returns differential test cases for R4.1-R4.4.
// R4.1: exit 0 on success. R4.2: exit 1 on errors.
// R4.3: differential tests compare file contents and exit codes.
// R4.4: comprehensive coverage of all flag combinations.
func buildR4Tests() []testutils.DiffTest {
	return []testutils.DiffTest{
		invalidLineCountTest(),
		invalidByteCountTest(),
		invalidChunkCountTest(),
		invalidOptionTest(),
		successExitCodeTest(),
	}
}

// invalidLineCountTest verifies R4.2: invalid -l count exits 1.
func invalidLineCountTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:      "invalid_line_count",
		Args:      []string{"-l", "0"},
		Stdin:     []byte("test\n"),
		ExitCode:  1,
		Normalize: []testutils.NormalizeFunc{clearOutput},
	}
}

// invalidByteCountTest verifies R4.2: invalid -b count exits 1.
func invalidByteCountTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:      "invalid_byte_count",
		Args:      []string{"-b", "abc"},
		Stdin:     []byte("test\n"),
		ExitCode:  1,
		Normalize: []testutils.NormalizeFunc{clearOutput},
	}
}

// invalidChunkCountTest verifies R4.2: invalid -n count exits 1.
func invalidChunkCountTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:      "invalid_chunk_count",
		Args:      []string{"-n", "0"},
		Stdin:     []byte("test\n"),
		ExitCode:  1,
		Normalize: []testutils.NormalizeFunc{clearOutput},
	}
}

// invalidOptionTest verifies R4.2: unrecognized option exits 1.
func invalidOptionTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:      "invalid_option",
		Args:      []string{"--nonexistent-option"},
		Stdin:     []byte("test\n"),
		ExitCode:  1,
		Normalize: []testutils.NormalizeFunc{clearOutput},
	}
}

// successExitCodeTest verifies R4.1: successful split exits 0.
func successExitCodeTest() testutils.DiffTest {
	return testutils.DiffTest{
		Name:     "success_exit_code",
		Args:     []string{"-l", "5"},
		Stdin:    generateLines(1, 10),
		ExitCode: 0,
		ExpectedFiles: map[string][]byte{
			"xaa": generateLines(1, 5),
			"xab": generateLines(6, 5),
		},
	}
}
