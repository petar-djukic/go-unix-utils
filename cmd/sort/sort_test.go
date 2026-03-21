// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd053-sort R1.1–R1.7, R2.1, R3.1–R3.4, R4.1–R4.4.
package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skipf("reference binary gsort not in PATH: %v", err)
	}

	// Create temp files for multi-file tests (R1.3).
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "a.txt")
	fileB := filepath.Join(tmpDir, "b.txt")
	writeTestFile(t, fileA, "banana\napple\n")
	writeTestFile(t, fileB, "cherry\ndate\n")

	tests := []testutils.DiffTest{
		// R1.1: default lexicographic sort.
		{
			Name:  "default_sort",
			Stdin: []byte("banana\napple\ncherry\n"),
		},
		// R1.1: already sorted input.
		{
			Name:  "already_sorted",
			Stdin: []byte("apple\nbanana\ncherry\n"),
		},
		// R1.1: single line input.
		{
			Name:  "single_line",
			Stdin: []byte("hello\n"),
		},
		// R1.1: empty input.
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		// R1.1: duplicate lines.
		{
			Name:  "duplicate_lines",
			Stdin: []byte("b\na\nb\na\n"),
		},
		// R1.1: case sensitivity (uppercase before lowercase in C locale).
		{
			Name:  "case_sensitivity",
			Stdin: []byte("b\nA\na\nB\n"),
		},
		// R1.2: read from stdin with no arguments.
		{
			Name:  "stdin_no_args",
			Stdin: []byte("z\ny\nx\n"),
		},
		// R1.2: read from stdin via explicit "-".
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("z\ny\nx\n"),
		},
		// R1.3: multiple input files combined and sorted.
		{
			Name: "multi_file",
			Args: []string{fileA, fileB},
		},
		// R1.3: file combined with stdin via "-".
		{
			Name:  "file_and_stdin",
			Args:  []string{fileA, "-"},
			Stdin: []byte("fig\nelderberry\n"),
		},
		// R1.4: reverse sort.
		{
			Name:  "reverse_sort",
			Args:  []string{"-r"},
			Stdin: []byte("apple\ncherry\nbanana\n"),
		},
		// R1.4: reverse with long flag.
		{
			Name:  "reverse_long_flag",
			Args:  []string{"--reverse"},
			Stdin: []byte("apple\ncherry\nbanana\n"),
		},
		// R1.4: reverse with duplicates.
		{
			Name:  "reverse_duplicates",
			Args:  []string{"-r"},
			Stdin: []byte("b\na\nb\na\n"),
		},
		// R1.1: lines with special characters.
		{
			Name:  "special_chars",
			Stdin: []byte("!\n@\n#\na\n1\n"),
		},
		// R1.1: lines with leading whitespace.
		{
			Name:  "leading_whitespace",
			Stdin: []byte(" b\na\n b\n"),
		},
		// R1.3: multiple files with reverse.
		{
			Name: "multi_file_reverse",
			Args: []string{"-r", fileA, fileB},
		},
		// R1.1: numeric strings sorted lexicographically.
		{
			Name:  "numeric_strings_lexico",
			Stdin: []byte("10\n2\n1\n20\n"),
		},
		// R1.2: stdin with empty lines.
		{
			Name:  "stdin_empty_lines",
			Stdin: []byte("\nb\n\na\n"),
		},
		// R1.5: unique removes duplicates.
		{
			Name:  "unique_basic",
			Args:  []string{"-u"},
			Stdin: []byte("b\na\nb\na\nc\n"),
		},
		// R1.5: unique with all identical lines.
		{
			Name:  "unique_all_same",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\na\n"),
		},
		// R1.5: unique with no duplicates.
		{
			Name:  "unique_no_dupes",
			Args:  []string{"-u"},
			Stdin: []byte("c\nb\na\n"),
		},
		// R1.5: unique with long flag.
		{
			Name:  "unique_long_flag",
			Args:  []string{"--unique"},
			Stdin: []byte("b\na\nb\n"),
		},
		// R1.5: unique combined with reverse.
		{
			Name:  "unique_reverse",
			Args:  []string{"-ru"},
			Stdin: []byte("b\na\nb\na\nc\n"),
		},
		// R1.5: unique with empty input.
		{
			Name:  "unique_empty",
			Args:  []string{"-u"},
			Stdin: []byte(""),
		},
		// R1.7: stable sort.
		{
			Name:  "stable_basic",
			Args:  []string{"-s"},
			Stdin: []byte("banana\napple\ncherry\n"),
		},
		// R1.7: stable with long flag.
		{
			Name:  "stable_long_flag",
			Args:  []string{"--stable"},
			Stdin: []byte("c\nb\na\n"),
		},
		// R1.7: stable combined with reverse.
		{
			Name:  "stable_reverse",
			Args:  []string{"-sr"},
			Stdin: []byte("banana\napple\ncherry\n"),
		},
		// R1.7: stable combined with unique.
		{
			Name:  "stable_unique",
			Args:  []string{"-su"},
			Stdin: []byte("b\na\nb\na\n"),
		},
		// R2.1: numeric sort.
		{
			Name:  "numeric_sort",
			Args:  []string{"-n"},
			Stdin: []byte("10\n2\n1\n20\n"),
		},
		// R2.1: numeric sort with negative numbers.
		{
			Name:  "numeric_negative",
			Args:  []string{"-n"},
			Stdin: []byte("3\n-1\n2\n-5\n"),
		},
		// R2.1: numeric sort with leading spaces.
		{
			Name:  "numeric_leading_spaces",
			Args:  []string{"-n"},
			Stdin: []byte("  10\n 2\n1\n"),
		},
		// R2.1: numeric with non-numeric lines.
		{
			Name:  "numeric_non_numeric",
			Args:  []string{"-n"},
			Stdin: []byte("abc\n10\n2\nxyz\n"),
		},
		// R2.1: numeric with reverse.
		{
			Name:  "numeric_reverse",
			Args:  []string{"-nr"},
			Stdin: []byte("10\n2\n1\n20\n"),
		},
		// R2.1: numeric unique.
		{
			Name:  "numeric_unique",
			Args:  []string{"-nu"},
			Stdin: []byte("3\n1\n2\n1\n3\n"),
		},
		// R3.2: sort by second whitespace-delimited field (AC1).
		{
			Name:  "key_field2",
			Args:  []string{"-k2,2"},
			Stdin: []byte("b 2\na 3\nc 1\n"),
		},
		// R3.1, R3.2: colon separator sort by second field (AC2).
		{
			Name:  "key_colon_sep",
			Args:  []string{"-t:", "-k2,2"},
			Stdin: []byte("root:0:root\nbin:1:bin\nnobody:99:nobody\n"),
		},
		// R3.1: colon separator, key from field 2 to end of line.
		{
			Name:  "key_colon_sep_open_end",
			Args:  []string{"-t:", "-k2"},
			Stdin: []byte("c:banana\na:cherry\nb:apple\n"),
		},
		// R3.3: multiple keys, alpha field 1 + numeric field 2 (AC3).
		{
			Name:  "multi_key_alpha_numeric",
			Args:  []string{"-k1,1", "-k2,2n"},
			Stdin: []byte("a 10\na 2\nb 1\na 3\n"),
		},
		// R3.3: multiple keys with all ties fall back to whole line.
		{
			Name:  "multi_key_all_tied",
			Args:  []string{"-k1,1", "-k2,2"},
			Stdin: []byte("a b\na b\nc d\n"),
		},
		// R3.2: character offsets within a field (AC4).
		{
			Name:  "key_char_offset",
			Args:  []string{"-k1.2,1.3"},
			Stdin: []byte("abc\nadc\naxc\naac\n"),
		},
		// R3.2: character offsets in second field.
		{
			Name:  "key_field2_char_offset",
			Args:  []string{"-k2.3,2.5"},
			Stdin: []byte("x abcde\ny abxyz\nz abaaa\n"),
		},
		// R3.2: key with reverse modifier per key.
		{
			Name:  "key_reverse_modifier",
			Args:  []string{"-k2,2r"},
			Stdin: []byte("a 2\nb 3\nc 1\n"),
		},
		// R3.2: key beyond available fields sorts as empty.
		{
			Name:  "key_beyond_fields",
			Args:  []string{"-k5,5"},
			Stdin: []byte("a b c\nd e f\ng h i\n"),
		},
		// R1.5 + R3.2: unique dedup by key.
		{
			Name:  "unique_with_key",
			Args:  []string{"-u", "-k1,1"},
			Stdin: []byte("a 2\na 1\nb 3\nb 4\n"),
		},
		// R3.2: key with --key= long form.
		{
			Name:  "key_long_flag",
			Args:  []string{"--key=2,2"},
			Stdin: []byte("b 2\na 3\nc 1\n"),
		},
		// R3.1: --field-separator= long form.
		{
			Name:  "sep_long_flag",
			Args:  []string{"--field-separator=:", "--key=2,2"},
			Stdin: []byte("root:0:root\nbin:1:bin\nnobody:99:nobody\n"),
		},
		// R3.3: three keys with mixed modifiers.
		{
			Name:  "three_keys",
			Args:  []string{"-t:", "-k1,1", "-k2,2n", "-k3,3"},
			Stdin: []byte("a:2:y\na:10:x\nb:1:z\na:2:x\n"),
		},
		// R3.4: -b ignore leading blanks on whole-line sort.
		{
			Name:  "ignore_blanks_basic",
			Args:  []string{"-b"},
			Stdin: []byte("  cherry\napple\n banana\n"),
		},
		// R3.4: -b with --ignore-leading-blanks long form.
		{
			Name:  "ignore_blanks_long_flag",
			Args:  []string{"--ignore-leading-blanks"},
			Stdin: []byte("  z\ny\n x\n"),
		},
		// R3.4: -b combined with -u removes duplicates ignoring blanks.
		{
			Name:  "ignore_blanks_unique",
			Args:  []string{"-bu"},
			Stdin: []byte("  a\na\n b\nb\n"),
		},
		// R3.4: -b combined with -r for reverse ignoring blanks.
		{
			Name:  "ignore_blanks_reverse",
			Args:  []string{"-br"},
			Stdin: []byte("  cherry\napple\n banana\n"),
		},
		// R3.2: b modifier on key start with -t separator.
		{
			Name:  "key_b_modifier_sep",
			Args:  []string{"-t:", "-k2b,2"},
			Stdin: []byte("a: cherry\nb: apple\nc: banana\n"),
		},
		// R3.2: b modifier on key with whitespace fields.
		{
			Name:  "key_b_modifier_whitespace",
			Args:  []string{"-b", "-k1,1"},
			Stdin: []byte("  banana\napple\n  cherry\n"),
		},
		// R3.2: f modifier folds case for comparison.
		{
			Name:  "key_f_modifier",
			Args:  []string{"-k1,1f"},
			Stdin: []byte("Banana\napple\nCherry\n"),
		},
		// R3.2: f modifier with mixed case dedup.
		{
			Name:  "key_f_modifier_unique",
			Args:  []string{"-u", "-k1,1f"},
			Stdin: []byte("Apple\napple\nBanana\nbanana\n"),
		},
		// R3.2: d modifier dictionary order.
		{
			Name:  "key_d_modifier",
			Args:  []string{"-k1,1d"},
			Stdin: []byte("a-c\na.b\nab\n"),
		},
		// R3.2: d modifier ignores special chars for sorting.
		{
			Name:  "key_d_modifier_special",
			Args:  []string{"-k1,1d"},
			Stdin: []byte("b!x\na@y\nc#z\n"),
		},
		// R3.2: i modifier ignores non-printing characters.
		{
			Name:  "key_i_modifier",
			Args:  []string{"-k1,1i"},
			Stdin: []byte("b\x01x\na\x02y\nc\x03z\n"),
		},
		// R3.2: i modifier with tab characters (printing, kept).
		{
			Name:  "key_i_modifier_tabs",
			Args:  []string{"-k1,1i"},
			Stdin: []byte("b\ty\na\tx\nc\tz\n"),
		},
		// R3.2: h modifier human-numeric sort on key.
		{
			Name:  "key_h_modifier",
			Args:  []string{"-k1,1h"},
			Stdin: []byte("10K\n2M\n1G\n500\n"),
		},
		// R3.2: h modifier with same suffix different values.
		{
			Name:  "key_h_modifier_same_suffix",
			Args:  []string{"-k1,1h"},
			Stdin: []byte("5K\n10K\n1K\n"),
		},
		// R3.2: M modifier month sort on key.
		{
			Name:  "key_M_modifier",
			Args:  []string{"-k1,1M"},
			Stdin: []byte("Mar\nJan\nFeb\nDec\n"),
		},
		// R3.2: M modifier with unknown months sorting first.
		{
			Name:  "key_M_modifier_unknown",
			Args:  []string{"-k1,1M"},
			Stdin: []byte("Jan\nfoo\nMar\nbar\n"),
		},
		// R3.2: V modifier version sort on key.
		{
			Name:  "key_V_modifier",
			Args:  []string{"-k1,1V"},
			Stdin: []byte("file10\nfile2\nfile1\nfile20\n"),
		},
		// R3.2: V modifier with dotted version numbers.
		{
			Name:  "key_V_modifier_dotted",
			Args:  []string{"-k1,1V"},
			Stdin: []byte("1.10.0\n1.2.0\n1.1.0\n2.0.0\n"),
		},
		// R3.3: multiple keys with different modifiers.
		{
			Name:  "multi_key_mixed_modifiers",
			Args:  []string{"-k1,1f", "-k2,2n"},
			Stdin: []byte("Apple 10\napple 2\nBanana 1\n"),
		},
		// R3.3: three keys with b on start, n, r modifiers.
		{
			Name:  "multi_key_bnr",
			Args:  []string{"-t:", "-k1b,1", "-k2,2n", "-k3,3r"},
			Stdin: []byte(" a:2:y\na:10:x\n b:1:z\na:2:x\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCheck tests -c/--check and -C/--check=quiet flags (R4.2).
func TestDiffCheck(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skipf("reference binary gsort not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.1, R4.2: -c on sorted input exits 0.
		{
			Name:     "check_sorted",
			Args:     []string{"-c"},
			Stdin:    []byte("a\nb\nc\n"),
			ExitCode: 0,
		},
		// R4.2: -c on unsorted input exits 1.
		{
			Name:      "check_unsorted",
			Args:      []string{"-c"},
			Stdin:     []byte("b\na\nc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryPrefix},
		},
		// R4.2: --check long flag on sorted input.
		{
			Name:     "check_long_flag_sorted",
			Args:     []string{"--check"},
			Stdin:    []byte("apple\nbanana\ncherry\n"),
			ExitCode: 0,
		},
		// R4.2: --check long flag on unsorted input.
		{
			Name:      "check_long_flag_unsorted",
			Args:      []string{"--check"},
			Stdin:     []byte("cherry\napple\nbanana\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryPrefix},
		},
		// R4.2: -C quiet mode on unsorted input exits 1, no stderr.
		{
			Name:     "check_quiet_unsorted",
			Args:     []string{"-C"},
			Stdin:    []byte("b\na\nc\n"),
			ExitCode: 1,
		},
		// R4.2: --check=quiet on unsorted input exits 1, no stderr.
		{
			Name:     "check_quiet_long_unsorted",
			Args:     []string{"--check=quiet"},
			Stdin:    []byte("z\na\nb\n"),
			ExitCode: 1,
		},
		// R4.2: --check=silent on unsorted input exits 1, no stderr.
		{
			Name:     "check_silent_long_unsorted",
			Args:     []string{"--check=silent"},
			Stdin:    []byte("z\na\nb\n"),
			ExitCode: 1,
		},
		// R4.2: -C on sorted input exits 0.
		{
			Name:     "check_quiet_sorted",
			Args:     []string{"-C"},
			Stdin:    []byte("a\nb\nc\n"),
			ExitCode: 0,
		},
		// R4.2: -c on single line (trivially sorted).
		{
			Name:     "check_single_line",
			Args:     []string{"-c"},
			Stdin:    []byte("hello\n"),
			ExitCode: 0,
		},
		// R4.2: -c on empty input (trivially sorted).
		{
			Name:     "check_empty",
			Args:     []string{"-c"},
			Stdin:    []byte(""),
			ExitCode: 0,
		},
		// R4.2: -c with -n on numerically sorted input.
		{
			Name:     "check_numeric_sorted",
			Args:     []string{"-c", "-n"},
			Stdin:    []byte("1\n2\n10\n"),
			ExitCode: 0,
		},
		// R4.2: -c with -n on numerically unsorted input.
		{
			Name:      "check_numeric_unsorted",
			Args:      []string{"-c", "-n"},
			Stdin:     []byte("1\n10\n2\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryPrefix},
		},
		// R4.2: -c with -r on reverse-sorted input.
		{
			Name:     "check_reverse_sorted",
			Args:     []string{"-c", "-r"},
			Stdin:    []byte("c\nb\na\n"),
			ExitCode: 0,
		},
		// R4.2: -c with -r on non-reverse-sorted input.
		{
			Name:      "check_reverse_unsorted",
			Args:      []string{"-c", "-r"},
			Stdin:     []byte("a\nb\nc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryPrefix},
		},
		// R4.2: -c with duplicate lines (sorted).
		{
			Name:     "check_duplicates_sorted",
			Args:     []string{"-c"},
			Stdin:    []byte("a\na\nb\nb\n"),
			ExitCode: 0,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffExitCodes tests exit code behavior (R4.1, R4.3).
func TestDiffExitCodes(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skipf("reference binary gsort not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.3: invalid short flag exits 2.
		{
			Name:      "invalid_short_flag",
			Args:      []string{"-Q"},
			Stdin:     []byte("a\n"),
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryPrefix},
		},
		// R4.3: invalid long flag exits 2.
		{
			Name:      "invalid_long_flag",
			Args:      []string{"--nonexistent"},
			Stdin:     []byte("a\n"),
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryPrefix},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffOutput tests -o/--output flag (R1.6).
func TestDiffOutput(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skipf("reference binary gsort not in PATH: %v", err)
	}

	tmpDir := t.TempDir()
	out1 := filepath.Join(tmpDir, "out1.txt")
	out2 := filepath.Join(tmpDir, "out2.txt")
	out3 := filepath.Join(tmpDir, "out3.txt")

	tests := []testutils.DiffTest{
		// R1.6: -o writes to file instead of stdout.
		{
			Name:    "output_short_flag",
			Args:    []string{"-o", out1},
			Stdin:   []byte("c\na\nb\n"),
			WorkDir: tmpDir,
		},
		// R1.6: --output=FILE long form.
		{
			Name:    "output_long_flag",
			Args:    []string{"--output=" + out2},
			Stdin:   []byte("z\ny\nx\n"),
			WorkDir: tmpDir,
		},
		// R1.6: -o combined with -u.
		{
			Name:    "output_with_unique",
			Args:    []string{"-u", "-o", out3},
			Stdin:   []byte("b\na\nb\na\n"),
			WorkDir: tmpDir,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)

	// Verify file contents produced by the go binary.
	verifyFileContent(t, out1, "a\nb\nc\n")
	verifyFileContent(t, out2, "x\ny\nz\n")
	verifyFileContent(t, out3, "a\nb\n")
}

// TestOutputInPlace tests -o with the same file as input (R1.6).
func TestOutputInPlace(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	dataFile := filepath.Join(tmpDir, "data.txt")
	writeTestFile(t, dataFile, "banana\napple\ncherry\n")

	cmd := exec.Command(goBin, "-o", dataFile, dataFile)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sort -o inplace failed: %v\noutput: %s", err, out)
	}

	verifyFileContent(t, dataFile, "apple\nbanana\ncherry\n")
}

// writeTestFile writes content to path, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}

// verifyFileContent checks that a file contains the expected content.
func verifyFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("file %s content mismatch:\ngot:  %q\nwant: %q", path, got, want)
	}
}

// normalizeBinaryPrefix normalizes the binary name prefix in stderr output.
// The reference binary may identify itself by full path (e.g.,
// "/opt/homebrew/bin/sort:") while our binary uses "sort:".
// Also normalizes the "Try '...--help'" line which contains the binary path.
func normalizeBinaryPrefix(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	for i, line := range lines {
		if idx := bytes.Index(line, []byte(": ")); idx >= 0 {
			lines[i] = append([]byte("sort"), line[idx:]...)
		} else if bytes.Contains(line, []byte("--help'")) {
			lines[i] = []byte("try 'sort --help' for more information.")
		}
	}
	return bytes.ToLower(bytes.Join(lines, []byte("\n")))
}
