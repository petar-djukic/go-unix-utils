// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/uniq against guniq (GNU coreutils).
// Covers prd028-uniq R1.1-R1.4: default adjacent-line deduplication,
// stdin/file input, output file, and exit code behavior.
// Covers prd028-uniq R2.1-R2.4: counting (-c), duplicate filtering (-d, -u),
// and all-repeated (-D) output modes.
// Covers prd028-uniq R3.1-R3.4: field skip (-f), char skip (-s), check-chars
// (-w), and case-insensitive (-i) comparison options.
// Covers prd028-uniq R4.1-R4.4: zero-terminated (-z), --group, --version, --help.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgNameNormalizer replaces the reference binary name (guniq or its
// full path) with the Go binary name (uniq) in stderr so error message
// comparisons match. Handles both "guniq:" prefixes and full paths like
// "/opt/homebrew/bin/guniq" in "Try '...' --help" messages.
func stderrProgNameNormalizer(data []byte) []byte {
	// First, replace full-path occurrences by finding path segments ending
	// in guniq (e.g., "'/opt/homebrew/bin/guniq" → "'uniq").
	for {
		pos := bytes.Index(data, []byte("guniq"))
		if pos < 0 {
			break
		}
		// Walk backwards to find the start of a filesystem path.
		start := pos
		for start > 0 && data[start-1] != '\'' && data[start-1] != '"' && data[start-1] != ' ' && data[start-1] != '\n' {
			start--
		}
		data = append(data[:start], append([]byte("uniq"), data[pos+len("guniq"):]...)...)
	}
	return data
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: suppress adjacent duplicate lines.
		{
			Name:  "adjacent_duplicates",
			Stdin: []byte("a\na\nb\na\n"),
		},
		// R1.1: all lines identical.
		{
			Name:  "all_identical",
			Stdin: []byte("x\nx\nx\nx\n"),
		},
		// R1.2: no duplicates — all lines unique.
		{
			Name:  "no_duplicates",
			Stdin: []byte("a\nb\nc\nd\n"),
		},
		// R1.2: single line input.
		{
			Name:  "single_line",
			Stdin: []byte("hello\n"),
		},
		// R1.1: empty input produces no output.
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		// R1.4: case-sensitive comparison — 'A' and 'a' are different.
		{
			Name:  "case_sensitive",
			Stdin: []byte("A\na\nA\n"),
		},
		// R1.1: multiple runs of duplicates.
		{
			Name:  "multiple_runs",
			Stdin: []byte("a\na\nb\nb\nc\nc\n"),
		},
		// R1.1: line without trailing newline at end of input.
		{
			Name:  "no_trailing_newline",
			Stdin: []byte("a\na\nb"),
		},
		// R1.2: non-adjacent duplicates are not suppressed.
		{
			Name:  "non_adjacent_duplicates",
			Stdin: []byte("a\nb\na\nb\n"),
		},
		// R1.1: blank lines are treated as identical adjacent lines.
		{
			Name:  "blank_lines",
			Stdin: []byte("\n\na\n\n\n"),
		},
		// R1.1: lines with leading/trailing spaces are compared exactly.
		{
			Name:  "whitespace_significant",
			Stdin: []byte("a \na\n a\n"),
		},
		// R1.3: read from stdin when '-' is given explicitly.
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("x\nx\ny\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCount tests R2.4: -c prefixes lines with occurrence count.
func TestDiffCount(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R2.4: count with mixed duplicates.
		{
			Name:  "count_mixed",
			Args:  []string{"-c"},
			Stdin: []byte("a\na\nb\na\n"),
		},
		// R2.4: count with single lines.
		{
			Name:  "count_single_lines",
			Args:  []string{"-c"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.4: count with all identical.
		{
			Name:  "count_all_identical",
			Args:  []string{"-c"},
			Stdin: []byte("x\nx\nx\nx\n"),
		},
		// R2.4: count with no trailing newline.
		{
			Name:  "count_no_trailing_newline",
			Args:  []string{"-c"},
			Stdin: []byte("a\na\nb"),
		},
		// R2.4: count with empty input.
		{
			Name:  "count_empty",
			Args:  []string{"-c"},
			Stdin: []byte(""),
		},
		// R2.4: --count long form.
		{
			Name:  "count_long_flag",
			Args:  []string{"--count"},
			Stdin: []byte("a\na\nb\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRepeated tests R2.1: -d prints only duplicate lines.
func TestDiffRepeated(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R2.1: only duplicated lines survive.
		{
			Name:  "repeated_mixed",
			Args:  []string{"-d"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.1: no duplicates — empty output.
		{
			Name:  "repeated_no_dups",
			Args:  []string{"-d"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.1: all identical — one output line.
		{
			Name:  "repeated_all_identical",
			Args:  []string{"-d"},
			Stdin: []byte("x\nx\nx\n"),
		},
		// R2.1: --repeated long form.
		{
			Name:  "repeated_long_flag",
			Args:  []string{"--repeated"},
			Stdin: []byte("a\na\nb\n"),
		},
		// R2.1: single line — not repeated, empty output.
		{
			Name:  "repeated_single_line",
			Args:  []string{"-d"},
			Stdin: []byte("hello\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffUnique tests R2.3: -u prints only unique (non-repeated) lines.
func TestDiffUnique(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R2.3: only unique lines survive.
		{
			Name:  "unique_mixed",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.3: no duplicates — all lines output.
		{
			Name:  "unique_all_unique",
			Args:  []string{"-u"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.3: all identical — empty output.
		{
			Name:  "unique_all_identical",
			Args:  []string{"-u"},
			Stdin: []byte("x\nx\nx\n"),
		},
		// R2.3: --unique long form.
		{
			Name:  "unique_long_flag",
			Args:  []string{"--unique"},
			Stdin: []byte("a\na\nb\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffAllRepeated tests R2.2/R2.4: -D prints all duplicate lines with
// optional delimiter methods (none, prepend, separate).
func TestDiffAllRepeated(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R2.2: -D prints all lines of duplicate groups.
		{
			Name:  "all_repeated_default",
			Args:  []string{"-D"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.4: --all-repeated=none (same as -D).
		{
			Name:  "all_repeated_none",
			Args:  []string{"--all-repeated=none"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.4: --all-repeated=prepend adds blank line before each group.
		{
			Name:  "all_repeated_prepend",
			Args:  []string{"--all-repeated=prepend"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.4: --all-repeated=separate adds blank line between groups.
		{
			Name:  "all_repeated_separate",
			Args:  []string{"--all-repeated=separate"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.2: no duplicates — empty output for -D.
		{
			Name:  "all_repeated_no_dups",
			Args:  []string{"-D"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.2: all identical — all lines output.
		{
			Name:  "all_repeated_all_identical",
			Args:  []string{"-D"},
			Stdin: []byte("x\nx\nx\n"),
		},
		// R2.4: --all-repeated bare (no =METHOD) defaults to none.
		{
			Name:  "all_repeated_bare_long",
			Args:  []string{"--all-repeated"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R2.4: prepend with single duplicate group.
		{
			Name:  "all_repeated_prepend_single_group",
			Args:  []string{"--all-repeated=prepend"},
			Stdin: []byte("a\nb\nb\nc\n"),
		},
		// R2.4: separate with multiple groups.
		{
			Name:  "all_repeated_separate_multi",
			Args:  []string{"--all-repeated=separate"},
			Stdin: []byte("a\na\nb\nc\nc\nd\nd\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCombinedFlags tests AC5: combined flag interactions (-c -d, -c -u).
func TestDiffCombinedFlags(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// AC5: -c -d counts only repeated groups.
		{
			Name:  "count_repeated",
			Args:  []string{"-c", "-d"},
			Stdin: []byte("a\na\nb\nc\nc\nc\n"),
		},
		// AC5: -c -u counts only unique groups.
		{
			Name:  "count_unique",
			Args:  []string{"-c", "-u"},
			Stdin: []byte("a\na\nb\nc\nc\nc\n"),
		},
		// AC5: -cd combined short form.
		{
			Name:  "count_repeated_short",
			Args:  []string{"-cd"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// AC5: -cu combined short form.
		{
			Name:  "count_unique_short",
			Args:  []string{"-cu"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestInputFile tests R1.2: reading from a named input file.
func TestInputFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputFile, []byte("a\na\nb\nc\nc\n"), 0o644); err != nil {
		t.Fatalf("writing input file: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:    "read_from_file",
			Args:    []string{inputFile},
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestOutputFile tests R1.3: writing to a named output file.
func TestOutputFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	// Create input file in a shared location, use separate dirs for output.
	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputFile, []byte("a\na\nb\nb\nc\n"), 0o644); err != nil {
		t.Fatalf("writing input file: %v", err)
	}

	goOutDir := filepath.Join(dir, "go_out")
	refOutDir := filepath.Join(dir, "ref_out")
	if err := os.Mkdir(goOutDir, 0o755); err != nil {
		t.Fatalf("creating go output dir: %v", err)
	}
	if err := os.Mkdir(refOutDir, 0o755); err != nil {
		t.Fatalf("creating ref output dir: %v", err)
	}

	goOutput := filepath.Join(goOutDir, "out.txt")
	refOutput := filepath.Join(refOutDir, "out.txt")

	// Run Go binary.
	goCmd := exec.Command(goBin, inputFile, goOutput)
	goCmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := goCmd.CombinedOutput(); err != nil {
		t.Fatalf("go binary failed: %v\n%s", err, out)
	}

	// Run reference binary.
	refCmd := exec.Command(refBin, inputFile, refOutput)
	refCmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := refCmd.CombinedOutput(); err != nil {
		t.Fatalf("ref binary failed: %v\n%s", err, out)
	}

	// Compare output files.
	goData, err := os.ReadFile(goOutput)
	if err != nil {
		t.Fatalf("reading go output: %v", err)
	}
	refData, err := os.ReadFile(refOutput)
	if err != nil {
		t.Fatalf("reading ref output: %v", err)
	}

	if string(goData) != string(refData) {
		t.Errorf("output file mismatch:\ngo:  %q\nref: %q", goData, refData)
	}
}

// TestDiffSkipFields tests R3.1: -f N skips N whitespace-delimited fields.
func TestDiffSkipFields(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.1: skip 1 field — lines differ only in first field.
		{
			Name:  "skip_fields_1",
			Args:  []string{"-f", "1"},
			Stdin: []byte("a foo\nb foo\nc bar\n"),
		},
		// R3.1: skip 2 fields.
		{
			Name:  "skip_fields_2",
			Args:  []string{"-f", "2"},
			Stdin: []byte("a b same\nc d same\ne f diff\n"),
		},
		// R3.1: skip more fields than exist — all lines compare as empty.
		{
			Name:  "skip_fields_more_than_exist",
			Args:  []string{"-f", "10"},
			Stdin: []byte("a b\nc d\ne f\n"),
		},
		// R3.1: long form --skip-fields=N.
		{
			Name:  "skip_fields_long",
			Args:  []string{"--skip-fields=1"},
			Stdin: []byte("x same\ny same\nz diff\n"),
		},
		// R3.1: skip fields with count.
		{
			Name:  "skip_fields_with_count",
			Args:  []string{"-f", "1", "-c"},
			Stdin: []byte("a foo\nb foo\nc bar\n"),
		},
		// R3.1: fields separated by tabs.
		{
			Name:  "skip_fields_tabs",
			Args:  []string{"-f", "1"},
			Stdin: []byte("a\tfoo\nb\tfoo\nc\tbar\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSkipChars tests R3.2: -s N skips N characters after field skip.
func TestDiffSkipChars(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.2: skip 1 character.
		{
			Name:  "skip_chars_1",
			Args:  []string{"-s", "1"},
			Stdin: []byte("aXX\nbXX\ncYY\n"),
		},
		// R3.2: skip more characters than line length.
		{
			Name:  "skip_chars_more_than_length",
			Args:  []string{"-s", "100"},
			Stdin: []byte("abc\ndef\n"),
		},
		// R3.2: long form --skip-chars=N.
		{
			Name:  "skip_chars_long",
			Args:  []string{"--skip-chars=2"},
			Stdin: []byte("xxfoo\nyyfoo\nzzbar\n"),
		},
		// R3.2: combine -f and -s: skip 1 field then 2 chars.
		{
			Name:  "skip_fields_and_chars",
			Args:  []string{"-f", "1", "-s", "2"},
			Stdin: []byte("a xxfoo\nb yyfoo\nc zzbar\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCheckChars tests R3.3: -w N compares at most N characters.
func TestDiffCheckChars(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.3: compare only first 3 characters.
		{
			Name:  "check_chars_3",
			Args:  []string{"-w", "3"},
			Stdin: []byte("foobar\nfoobaz\nbarqux\n"),
		},
		// R3.3: check-chars larger than line — same as no limit.
		{
			Name:  "check_chars_large",
			Args:  []string{"-w", "100"},
			Stdin: []byte("abc\nabc\ndef\n"),
		},
		// R3.3: long form --check-chars=N.
		{
			Name:  "check_chars_long",
			Args:  []string{"--check-chars=2"},
			Stdin: []byte("ab1\nab2\ncd3\n"),
		},
		// R3.3: -w with -c.
		{
			Name:  "check_chars_with_count",
			Args:  []string{"-w", "3", "-c"},
			Stdin: []byte("foobar\nfoobaz\nbarqux\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffIgnoreCase tests R3.4: -i case-insensitive comparison.
func TestDiffIgnoreCase(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.4: case folding treats A and a as same.
		{
			Name:  "ignore_case_basic",
			Args:  []string{"-i"},
			Stdin: []byte("A\na\nb\n"),
		},
		// R3.4: mixed case runs.
		{
			Name:  "ignore_case_mixed",
			Args:  []string{"-i"},
			Stdin: []byte("Hello\nhello\nHELLO\nWorld\n"),
		},
		// R3.4: long form --ignore-case.
		{
			Name:  "ignore_case_long",
			Args:  []string{"--ignore-case"},
			Stdin: []byte("FOO\nfoo\nbar\n"),
		},
		// R3.4: -i with -c.
		{
			Name:  "ignore_case_with_count",
			Args:  []string{"-i", "-c"},
			Stdin: []byte("A\na\nB\n"),
		},
		// R3.4: -i with -d — only case-insensitive duplicates.
		{
			Name:  "ignore_case_with_repeated",
			Args:  []string{"-i", "-d"},
			Stdin: []byte("A\na\nb\nC\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCombinedComparisonFlags tests AC7: combined comparison flags work together.
func TestDiffCombinedComparisonFlags(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// AC7: -f 1 -s 2 -w 5 -i -c all combined.
		{
			Name:  "combined_all_comparison",
			Args:  []string{"-f", "1", "-s", "2", "-w", "5", "-i", "-c"},
			Stdin: []byte("x aaHELLOworld\ny bbhelloearth\nz ccDIFFERENT\n"),
		},
		// Combined: -f 1 -i.
		{
			Name:  "skip_fields_ignore_case",
			Args:  []string{"-f", "1", "-i"},
			Stdin: []byte("a FOO\nb foo\nc bar\n"),
		},
		// Combined: -s 1 -w 3.
		{
			Name:  "skip_chars_check_chars",
			Args:  []string{"-s", "1", "-w", "3"},
			Stdin: []byte("xfoobar\nyfooXXX\nzbarYYY\n"),
		},
		// Combined: -f 1 -w 3 -d — only duplicates with field skip and check chars.
		{
			Name:  "skip_fields_check_chars_repeated",
			Args:  []string{"-f", "1", "-w", "3", "-d"},
			Stdin: []byte("a foobar\nb foobaz\nc barqux\n"),
		},
		// Combined: -i with -D.
		{
			Name:  "ignore_case_all_repeated",
			Args:  []string{"-i", "-D"},
			Stdin: []byte("A\na\nb\nC\nc\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestFileNotFound tests R1.4: exit non-zero when input file does not exist.
func TestFileNotFound(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "nonexistent_file",
			Args:      []string{"/nonexistent/path/to/file.txt"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffZeroTerminated tests R4.1: -z/--zero-terminated uses NUL as delimiter.
func TestDiffZeroTerminated(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.1: basic NUL-delimited deduplication.
		{
			Name:  "zero_terminated_basic",
			Args:  []string{"-z"},
			Stdin: []byte("a\x00a\x00b\x00a\x00"),
		},
		// R4.1: NUL-delimited with all identical.
		{
			Name:  "zero_terminated_all_identical",
			Args:  []string{"-z"},
			Stdin: []byte("x\x00x\x00x\x00"),
		},
		// R4.1: NUL-delimited with count.
		{
			Name:  "zero_terminated_count",
			Args:  []string{"-z", "-c"},
			Stdin: []byte("a\x00a\x00b\x00"),
		},
		// R4.1: NUL-delimited with -d.
		{
			Name:  "zero_terminated_repeated",
			Args:  []string{"-z", "-d"},
			Stdin: []byte("a\x00a\x00b\x00c\x00c\x00"),
		},
		// R4.1: NUL-delimited with -u.
		{
			Name:  "zero_terminated_unique",
			Args:  []string{"-z", "-u"},
			Stdin: []byte("a\x00a\x00b\x00c\x00c\x00"),
		},
		// R4.1: NUL-delimited with -D.
		{
			Name:  "zero_terminated_all_repeated",
			Args:  []string{"-z", "-D"},
			Stdin: []byte("a\x00a\x00b\x00c\x00c\x00"),
		},
		// R4.1: long form --zero-terminated.
		{
			Name:  "zero_terminated_long",
			Args:  []string{"--zero-terminated"},
			Stdin: []byte("a\x00a\x00b\x00"),
		},
		// R4.1: NUL-delimited with -i.
		{
			Name:  "zero_terminated_ignore_case",
			Args:  []string{"-z", "-i"},
			Stdin: []byte("A\x00a\x00b\x00"),
		},
		// R4.1: lines contain embedded newlines when using -z.
		{
			Name:  "zero_terminated_embedded_newlines",
			Args:  []string{"-z"},
			Stdin: []byte("a\nb\x00a\nb\x00c\x00"),
		},
		// R4.1: empty NUL-delimited input.
		{
			Name:  "zero_terminated_empty",
			Args:  []string{"-z"},
			Stdin: []byte(""),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffGroup tests R4.2: --group inserts separators around groups.
func TestDiffGroup(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.2: --group=separate (between groups only).
		{
			Name:  "group_separate",
			Args:  []string{"--group=separate"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R4.2: --group=prepend (before each group).
		{
			Name:  "group_prepend",
			Args:  []string{"--group=prepend"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R4.2: --group=append (after each group).
		{
			Name:  "group_append",
			Args:  []string{"--group=append"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R4.2: --group=both (before and after each group, single separator between).
		{
			Name:  "group_both",
			Args:  []string{"--group=both"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R4.2: --group bare defaults to separate.
		{
			Name:  "group_bare",
			Args:  []string{"--group"},
			Stdin: []byte("a\na\nb\nc\nc\n"),
		},
		// R4.2: --group with all unique lines.
		{
			Name:  "group_all_unique",
			Args:  []string{"--group=separate"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R4.2: --group with single line.
		{
			Name:  "group_single_line",
			Args:  []string{"--group=prepend"},
			Stdin: []byte("hello\n"),
		},
		// R4.2: --group=both with single group.
		{
			Name:  "group_both_single_group",
			Args:  []string{"--group=both"},
			Stdin: []byte("a\na\na\n"),
		},
		// R4.2: --group with empty input.
		{
			Name:  "group_empty",
			Args:  []string{"--group=separate"},
			Stdin: []byte(""),
		},
		// R4.2: --group with -i.
		{
			Name:  "group_ignore_case",
			Args:  []string{"--group=separate", "-i"},
			Stdin: []byte("A\na\nb\nB\n"),
		},
		// R4.2: --group with -z.
		{
			Name:  "group_zero_terminated",
			Args:  []string{"--group=separate", "-z"},
			Stdin: []byte("a\x00a\x00b\x00c\x00c\x00"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffGroupIncompatible tests R4.3: --group is mutually exclusive with -c/-d/-D/-u.
func TestDiffGroupIncompatible(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guniq")
	if err != nil {
		t.Skipf("reference binary guniq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.3: --group with -c.
		{
			Name:      "group_with_count",
			Args:      []string{"--group", "-c"},
			Stdin:     []byte("a\na\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R4.3: --group with -d.
		{
			Name:      "group_with_repeated",
			Args:      []string{"--group", "-d"},
			Stdin:     []byte("a\na\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R4.3: --group with -D.
		{
			Name:      "group_with_all_repeated",
			Args:      []string{"--group", "-D"},
			Stdin:     []byte("a\na\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
		// R4.3: --group with -u.
		{
			Name:      "group_with_unique",
			Args:      []string{"--group", "-u"},
			Stdin:     []byte("a\na\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgNameNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffVersionHelp tests R4.4: --version and --help flags.
func TestDiffVersionHelp(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// --version: just check it exits 0 and produces output.
	t.Run("version", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(goBin, "--version")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("--version failed: %v", err)
		}
		if len(out) == 0 {
			t.Error("--version produced no output")
		}
	})

	// --help: just check it exits 0 and produces output.
	t.Run("help", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(goBin, "--help")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("--help failed: %v", err)
		}
		if len(out) == 0 {
			t.Error("--help produced no output")
		}
	})
}
