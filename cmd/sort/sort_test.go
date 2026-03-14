// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd053-sort R1.1, R1.2, R1.3, R1.4, R1.5, R1.6, R1.7, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4 differential tests
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

	tests := []testutils.DiffTest{
		// R1.1: Default lexicographic sort under LC_ALL=C.
		{
			Name:  "default_sort",
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Empty input.
		{
			Name:  "empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Single line.
		{
			Name:  "single_line",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Already sorted input.
		{
			Name:  "already_sorted",
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Case sensitivity under LC_ALL=C (uppercase before lowercase).
		{
			Name:  "case_sensitive_sort",
			Stdin: []byte("b\nA\na\nB\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Lines with special characters.
		{
			Name:  "special_chars",
			Stdin: []byte("!\n@\n#\nz\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Duplicate lines.
		{
			Name:  "duplicate_lines",
			Stdin: []byte("b\na\nb\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: Read from stdin with "-" argument.
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("c\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: -r reverses sort order.
		{
			Name:  "reverse",
			Args:  []string{"-r"},
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: --reverse long form.
		{
			Name:  "reverse_long",
			Args:  []string{"--reverse"},
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: -u removes duplicate lines.
		{
			Name:  "unique",
			Args:  []string{"-u"},
			Stdin: []byte("b\na\nb\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: --unique long form.
		{
			Name:  "unique_long",
			Args:  []string{"--unique"},
			Stdin: []byte("c\nc\na\nb\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4 + R1.5: -ru combined.
		{
			Name:  "reverse_unique",
			Args:  []string{"-ru"},
			Stdin: []byte("b\na\nb\nc\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Lines with leading/trailing whitespace.
		{
			Name:  "whitespace_lines",
			Stdin: []byte(" b\na\n b\n a\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Numeric strings sorted lexicographically.
		{
			Name:  "numeric_lex_sort",
			Stdin: []byte("10\n2\n1\n20\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Empty lines sort first.
		{
			Name:  "empty_lines",
			Stdin: []byte("b\n\na\n\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: -u with all identical lines.
		{
			Name:  "unique_all_same",
			Args:  []string{"-u"},
			Stdin: []byte("a\na\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.5: -u with already unique input.
		{
			Name:  "unique_already_unique",
			Args:  []string{"-u"},
			Stdin: []byte("c\na\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.7: -s stable sort preserves input order of equal lines.
		{
			Name:  "stable_sort",
			Args:  []string{"-s"},
			Stdin: []byte("banana\napple\ncherry\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.7: --stable long form.
		{
			Name:  "stable_sort_long",
			Args:  []string{"--stable"},
			Stdin: []byte("cherry\napple\nbanana\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.7: -s combined with -r.
		{
			Name:  "stable_reverse",
			Args:  []string{"-s", "-r"},
			Stdin: []byte("apple\ncherry\nbanana\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.7: -s combined with -u.
		{
			Name:  "stable_unique",
			Args:  []string{"-su"},
			Stdin: []byte("b\na\nb\nc\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -n numeric sort.
		{
			Name:  "numeric_sort",
			Args:  []string{"-n"},
			Stdin: []byte("10\n2\n1\n20\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -n with negative numbers.
		{
			Name:  "numeric_sort_negative",
			Args:  []string{"-n"},
			Stdin: []byte("5\n-3\n0\n-10\n3\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -n with decimal numbers.
		{
			Name:  "numeric_sort_decimal",
			Args:  []string{"-n"},
			Stdin: []byte("1.5\n1.2\n1.10\n0.5\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -n with non-numeric lines (treated as 0).
		{
			Name:  "numeric_sort_nonnumeric",
			Args:  []string{"-n"},
			Stdin: []byte("abc\n5\ndef\n0\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -n with leading whitespace.
		{
			Name:  "numeric_sort_whitespace",
			Args:  []string{"-n"},
			Stdin: []byte("  10\n2\n  1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: --numeric-sort long form.
		{
			Name:  "numeric_sort_long",
			Args:  []string{"--numeric-sort"},
			Stdin: []byte("10\n2\n1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -nr numeric reverse.
		{
			Name:  "numeric_reverse",
			Args:  []string{"-nr"},
			Stdin: []byte("10\n2\n1\n20\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: -nu numeric unique.
		{
			Name:  "numeric_unique",
			Args:  []string{"-nu"},
			Stdin: []byte("1\n01\n2\n02\n3\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -h human-numeric sort.
		{
			Name:  "human_numeric_sort",
			Args:  []string{"-h"},
			Stdin: []byte("1K\n2M\n500\n1G\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -h with mixed suffixes.
		{
			Name:  "human_numeric_mixed",
			Args:  []string{"-h"},
			Stdin: []byte("1.5K\n1K\n2K\n500\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: --human-numeric-sort long form.
		{
			Name:  "human_numeric_long",
			Args:  []string{"--human-numeric-sort"},
			Stdin: []byte("1G\n1M\n1K\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: -h with no suffix (plain numbers).
		{
			Name:  "human_numeric_no_suffix",
			Args:  []string{"-h"},
			Stdin: []byte("100\n10\n1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -M month sort.
		{
			Name:  "month_sort",
			Args:  []string{"-M"},
			Stdin: []byte("MAR\nJAN\nFEB\nDEC\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -M with lowercase months.
		{
			Name:  "month_sort_lowercase",
			Args:  []string{"-M"},
			Stdin: []byte("mar\njan\nfeb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -M unknown strings sort before JAN.
		{
			Name:  "month_sort_unknown",
			Args:  []string{"-M"},
			Stdin: []byte("JAN\nXYZ\nFEB\nAAA\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: --month-sort long form.
		{
			Name:  "month_sort_long",
			Args:  []string{"--month-sort"},
			Stdin: []byte("DEC\nJAN\nJUL\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: -M all months.
		{
			Name:  "month_sort_all",
			Args:  []string{"-M"},
			Stdin: []byte("DEC\nJUN\nMAR\nSEP\nJAN\nJUL\nAPR\nOCT\nFEB\nAUG\nMAY\nNOV\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: -V version sort.
		{
			Name:  "version_sort",
			Args:  []string{"-V"},
			Stdin: []byte("file10\nfile2\nfile1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: -V with dotted versions.
		{
			Name:  "version_sort_dotted",
			Args:  []string{"-V"},
			Stdin: []byte("1.10\n1.2\n1.1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: --version-sort long form.
		{
			Name:  "version_sort_long",
			Args:  []string{"--version-sort"},
			Stdin: []byte("a2\na10\na1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: -V with complex version strings.
		{
			Name:  "version_sort_complex",
			Args:  []string{"-V"},
			Stdin: []byte("lib-1.10.0\nlib-1.2.0\nlib-1.9.0\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: -V reverse.
		{
			Name:  "version_sort_reverse",
			Args:  []string{"-Vr"},
			Stdin: []byte("file1\nfile10\nfile2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1, R3.2: -t with -k and -n: numeric sort on second colon-delimited field.
		{
			Name:  "key_numeric_colon",
			Args:  []string{"-t:", "-k2,2", "-n"},
			Stdin: []byte("c:10\na:2\nb:1\nd:20\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: -k with per-key numeric modifier.
		{
			Name:  "key_perkey_numeric",
			Args:  []string{"-t:", "-k2,2n"},
			Stdin: []byte("c:10\na:2\nb:1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1, R3.2: -t with -k and -h: human-numeric sort on key field.
		{
			Name:  "key_human_numeric",
			Args:  []string{"-t:", "-k2,2", "-h"},
			Stdin: []byte("a:1K\nb:2M\nc:500\nd:1G\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: -k with per-key human-numeric modifier.
		{
			Name:  "key_perkey_human",
			Args:  []string{"-t:", "-k2,2h"},
			Stdin: []byte("a:1G\nb:1M\nc:1K\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1, R3.2: -t with -k and -M: month sort on key field.
		{
			Name:  "key_month",
			Args:  []string{"-t:", "-k2,2", "-M"},
			Stdin: []byte("a:MAR\nb:JAN\nc:FEB\nd:DEC\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: -k with per-key month modifier.
		{
			Name:  "key_perkey_month",
			Args:  []string{"-t:", "-k2,2M"},
			Stdin: []byte("x:DEC\ny:JAN\nz:JUL\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1, R3.2: -t with -k and -V: version sort on key field.
		{
			Name:  "key_version",
			Args:  []string{"-t:", "-k2,2", "-V"},
			Stdin: []byte("a:1.10\nb:1.2\nc:1.1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: -k with per-key version modifier.
		{
			Name:  "key_perkey_version",
			Args:  []string{"-t:", "-k2,2V"},
			Stdin: []byte("a:file10\nb:file2\nc:file1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.3: Multiple -k options. First key takes precedence, second breaks ties.
		{
			Name:  "multi_key",
			Args:  []string{"-t:", "-k1,1", "-k2,2n"},
			Stdin: []byte("b:2\na:10\na:3\nb:1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: -k with reverse per-key modifier.
		{
			Name:  "key_perkey_reverse",
			Args:  []string{"-t:", "-k2,2nr"},
			Stdin: []byte("a:10\nb:2\nc:1\nd:20\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.4: -b ignore leading blanks with key sort.
		{
			Name:  "key_ignore_blanks",
			Args:  []string{"-b", "-k2,2"},
			Stdin: []byte("1  b\n2 a\n3   c\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: -k without -t uses blank-separated fields, numeric.
		{
			Name:  "key_blank_fields_numeric",
			Args:  []string{"-k2,2", "-n"},
			Stdin: []byte("a 10\nb 2\nc 1\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1, R3.2: -k with global -n and -u.
		{
			Name:  "key_numeric_unique",
			Args:  []string{"-t:", "-k2,2", "-nu"},
			Stdin: []byte("a:1\nb:01\nc:2\nd:02\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.1: Exit 0 on successful sort (implicit in all above tests).
		// R4.2: -c check sorted input exits 0.
		{
			Name:  "check_sorted",
			Args:  []string{"-c"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.2: -c check unsorted input exits 1 with diagnostic.
		{
			Name:      "check_unsorted",
			Args:      []string{"-c"},
			Stdin:     []byte("c\na\nb\n"),
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R4.2: -C (check=quiet) unsorted input exits 1 without diagnostic.
		{
			Name:     "check_quiet_unsorted",
			Args:     []string{"-C"},
			Stdin:    []byte("c\na\nb\n"),
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
		},
		// R4.2: -C sorted input exits 0.
		{
			Name:  "check_quiet_sorted",
			Args:  []string{"-C"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.2: --check long form with unsorted input.
		{
			Name:      "check_long_unsorted",
			Args:      []string{"--check"},
			Stdin:     []byte("b\na\n"),
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R4.2: --check=quiet long form.
		{
			Name:     "check_quiet_long",
			Args:     []string{"--check=quiet"},
			Stdin:    []byte("b\na\n"),
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
		},
		// R4.2: --check=diagnose-first long form.
		{
			Name:      "check_diagnose_first",
			Args:      []string{"--check=diagnose-first"},
			Stdin:     []byte("z\na\n"),
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R4.2: -c with empty input (trivially sorted).
		{
			Name:  "check_empty",
			Args:  []string{"-c"},
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.2: -c with single line (trivially sorted).
		{
			Name:  "check_single_line",
			Args:  []string{"-c"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.2: -c with -n numeric check.
		{
			Name:  "check_numeric_sorted",
			Args:  []string{"-c", "-n"},
			Stdin: []byte("1\n2\n10\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.2: -c with -n numeric check unsorted.
		{
			Name:      "check_numeric_unsorted",
			Args:      []string{"-c", "-n"},
			Stdin:     []byte("1\n10\n2\n"),
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R4.2: -c with -r reverse check.
		{
			Name:  "check_reverse_sorted",
			Args:  []string{"-c", "-r"},
			Stdin: []byte("c\nb\na\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.2: -c with -r reverse check unsorted.
		{
			Name:      "check_reverse_unsorted",
			Args:      []string{"-c", "-r"},
			Stdin:     []byte("a\nb\nc\n"),
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R4.3: Invalid flag exits 2.
		{
			Name:      "invalid_flag",
			Args:      []string{"--invalid-xyz"},
			Stdin:     []byte(""),
			ExitCode:  2,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R4.3: Invalid short flag exits 2.
		{
			Name:      "invalid_short_flag",
			Args:      []string{"-Z"},
			Stdin:     []byte(""),
			ExitCode:  2,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFileInput tests reading from named files (R1.3).
func TestDiffFileInput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skipf("reference binary gsort not in PATH: %v", err)
	}

	tmpDir := t.TempDir()

	// Create test input files.
	file1 := filepath.Join(tmpDir, "input1.txt")
	file2 := filepath.Join(tmpDir, "input2.txt")
	if err := os.WriteFile(file1, []byte("cherry\napple\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("banana\ndate\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.3: Single file input.
		{
			Name: "single_file",
			Args: []string{file1},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: Multiple file inputs merged.
		{
			Name: "multi_file",
			Args: []string{file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffOutputFile tests -o flag for output to file (R1.6).
func TestDiffOutputFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skipf("reference binary gsort not in PATH: %v", err)
	}

	// Test -o writing to a separate output file.
	t.Run("output_to_file", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		inputFile := filepath.Join(tmpDir, "input.txt")
		goOut := filepath.Join(tmpDir, "go_out.txt")
		refOut := filepath.Join(tmpDir, "ref_out.txt")

		inputData := []byte("cherry\napple\nbanana\n")
		if err := os.WriteFile(inputFile, inputData, 0o644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}

		// Run Go binary.
		goCmd := exec.Command(goBin, "-o", goOut, inputFile)
		goCmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := goCmd.CombinedOutput(); err != nil {
			t.Fatalf("go binary failed: %v\n%s", err, out)
		}

		// Run reference binary.
		refCmd := exec.Command(refBin, "-o", refOut, inputFile)
		refCmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := refCmd.CombinedOutput(); err != nil {
			t.Fatalf("ref binary failed: %v\n%s", err, out)
		}

		goData, err := os.ReadFile(goOut)
		if err != nil {
			t.Fatalf("reading go output: %v", err)
		}
		refData, err := os.ReadFile(refOut)
		if err != nil {
			t.Fatalf("reading ref output: %v", err)
		}

		if string(goData) != string(refData) {
			t.Errorf("-o output mismatch:\nref: %q\ngot: %q", refData, goData)
		}
	})

	// Test -o with in-place sorting (output file same as input file).
	t.Run("inplace_sort", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		goFile := filepath.Join(tmpDir, "go_inplace.txt")
		refFile := filepath.Join(tmpDir, "ref_inplace.txt")

		inputData := []byte("cherry\napple\nbanana\n")
		if err := os.WriteFile(goFile, inputData, 0o644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}
		if err := os.WriteFile(refFile, inputData, 0o644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}

		// Run Go binary with in-place sort.
		goCmd := exec.Command(goBin, "-o", goFile, goFile)
		goCmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := goCmd.CombinedOutput(); err != nil {
			t.Fatalf("go binary failed: %v\n%s", err, out)
		}

		// Run reference binary with in-place sort.
		refCmd := exec.Command(refBin, "-o", refFile, refFile)
		refCmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := refCmd.CombinedOutput(); err != nil {
			t.Fatalf("ref binary failed: %v\n%s", err, out)
		}

		goData, err := os.ReadFile(goFile)
		if err != nil {
			t.Fatalf("reading go output: %v", err)
		}
		refData, err := os.ReadFile(refFile)
		if err != nil {
			t.Fatalf("reading ref output: %v", err)
		}

		if string(goData) != string(refData) {
			t.Errorf("-o inplace mismatch:\nref: %q\ngot: %q", refData, goData)
		}
	})
}

// stderrBinaryName matches the binary name prefix in stderr messages
// (e.g., "gsort:" or "sort:" or a full path to the binary).
var stderrBinaryName = regexp.MustCompile(`(?m)^[^\s:]+:`)

// stderrTryHelp matches the "Try '...' for more information." line
// that GNU coreutils prints after error messages.
var stderrTryHelp = regexp.MustCompile(`(?m)^Try '.*' for more information\.\n?`)

// normalizeStderr replaces the binary name prefix in stderr and removes
// the "Try ... --help" line so that differences between "sort:" and
// "gsort:" do not cause false failures.
func normalizeStderr(b []byte) []byte {
	b = stderrBinaryName.ReplaceAll(b, []byte("SORT:"))
	b = stderrTryHelp.ReplaceAll(b, nil)
	return b
}
