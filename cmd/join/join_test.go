// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces "gjoin:" with "join:" so stderr messages
// from the reference binary match our binary's program name.
func normalizeProgramName(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gjoin:"), []byte("join:"))
}

// normalizeFileError normalizes file-open error messages across platforms.
func normalizeFileError(b []byte) []byte {
	return bytes.ToLower(b)
}

// writeTestFiles creates file1.txt and file2.txt in a temp directory.
func writeTestFiles(t *testing.T, content1, content2 string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte(content1), 0o644); err != nil {
		t.Fatalf("writing file1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte(content2), 0o644); err != nil {
		t.Fatalf("writing file2.txt: %v", err)
	}
	return dir
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gjoin")
	if err != nil {
		t.Skipf("reference binary gjoin not in PATH: %v", err)
	}

	// Setup test file pairs for different scenarios.
	dirAllMatch := writeTestFiles(t, "a 1\nb 2\nc 3\n", "a X\nb Y\nc Z\n")
	dirPartial := writeTestFiles(t, "a 1\nb 2\nc 3\n", "b Y\nc Z\nd W\n")
	dirNoMatch := writeTestFiles(t, "a 1\nc 3\n", "b 2\nd 4\n")
	dirEmpty1 := writeTestFiles(t, "", "a X\nb Y\n")
	dirEmpty2 := writeTestFiles(t, "a 1\nb 2\n", "")
	dirBothEmpty := writeTestFiles(t, "", "")
	dirMultiField := writeTestFiles(t, "a 1 2\nb 3 4\n", "a X Y\nb Z W\n")
	dirDupKeys := writeTestFiles(t, "a 1\na 2\n", "a X\na Y\n")
	dirSingleField := writeTestFiles(t, "a\nb\nc\n", "a\nc\n")

	// R2.1: -1/-2 field selection test data.
	dirField12 := writeTestFiles(t, "X a 1\nY b 2\n", "a P\nb Q\n")
	// R2.2: -j combined field test data.
	dirFieldJ := writeTestFiles(t, "1 a\n2 b\n", "1 X\n2 Y\n")
	// R2.3: -o output format test data.
	dirOutputFmt := writeTestFiles(t, "a 1 2\nb 3 4\n", "a X Y\nb Z W\n")
	// R2.4: -t custom separator test data.
	dirCommaSep := writeTestFiles(t, "a,1,2\nb,3,4\n", "a,X,Y\nb,Z,W\n")
	// R2.1 + R2.4 combined: join on non-first field with custom separator.
	dirCommaField := writeTestFiles(t, "1,a\n2,b\n", "a,P\nb,Q\n")

	// R3.1: -a unpairable lines test data.
	dirUnpair := writeTestFiles(t, "a 1\nb 2\nc 3\n", "b Y\nc Z\nd W\n")
	// R3.2: -v unpairable only test data.
	dirVOnly := writeTestFiles(t, "a 1\nb 2\nc 3\n", "b Y\nd W\n")
	// R3.3: -e empty replacement test data.
	dirEmptyRepl := writeTestFiles(t, "a 1\nb 2\nc 3\n", "a X\nc Z\n")
	// R3.4: --header test data (headers may not be sorted relative to data).
	dirHeader := writeTestFiles(t, "NAME VAL\na 1\nb 2\n", "NAME CODE\na X\nb Y\n")
	dirHeaderUnsorted := writeTestFiles(t, "ZZZ VAL\na 1\nb 2\n", "ZZZ CODE\na X\nb Y\n")

	errNorm := []testutils.NormalizeFunc{normalizeProgramName, normalizeFileError}

	tests := []testutils.DiffTest{
		// R1.1: Join lines where first field matches, one output line per pair.
		{
			Name:    "R1.1_default_join_all_match",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirAllMatch,
		},
		{
			Name:    "R1.1_partial_match",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirPartial,
		},
		{
			Name:    "R1.1_no_matching_keys",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirNoMatch,
		},
		{
			Name:    "R1.1_duplicate_keys_cartesian",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirDupKeys,
		},
		{
			Name:    "R1.1_single_field_lines",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingleField,
		},

		// R1.2: Whitespace field separator, space output separator.
		{
			Name:    "R1.2_multiple_fields_space_output",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirMultiField,
		},

		// R1.3: Unpairable lines suppressed by default.
		{
			Name:    "R1.3_file1_empty_all_suppressed",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirEmpty1,
		},
		{
			Name:    "R1.3_file2_empty_all_suppressed",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirEmpty2,
		},
		{
			Name:    "R1.3_both_empty",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirBothEmpty,
		},

		// R1.4: stdin as '-' for one of the file arguments.
		{
			Name:    "R1.4_stdin_as_file1",
			Args:    []string{"-", "file2.txt"},
			Stdin:   []byte("a 1\nb 2\nc 3\n"),
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirAllMatch,
		},
		{
			Name:    "R1.4_stdin_as_file2",
			Args:    []string{"file1.txt", "-"},
			Stdin:   []byte("a X\nb Y\nc Z\n"),
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirAllMatch,
		},

		// R2.1: -1 FIELD and -2 FIELD join on specified fields.
		{
			Name:    "R2.1_field_selection_1_2",
			Args:    []string{"-1", "2", "-2", "1", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirField12,
		},

		// R2.2: -j FIELD sets join field for both files.
		{
			Name:    "R2.2_combined_field_j",
			Args:    []string{"-j", "1", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirFieldJ,
		},
		{
			Name:    "R2.2_j_field2",
			Args:    []string{"-j", "2", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: writeTestFiles(t, "X a\nY b\n", "P a\nQ b\n"),
		},

		// R2.3: -o FORMAT output field selection.
		{
			Name:    "R2.3_output_format_join_field",
			Args:    []string{"-o", "0", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirOutputFmt,
		},
		{
			Name:    "R2.3_output_format_specific_fields",
			Args:    []string{"-o", "1.2,2.2", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirOutputFmt,
		},
		{
			Name:    "R2.3_output_format_mixed",
			Args:    []string{"-o", "0,1.2,2.2,1.3,2.3", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirOutputFmt,
		},

		// R2.4: -t CHAR custom field separator.
		{
			Name:    "R2.4_comma_separator",
			Args:    []string{"-t", ",", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirCommaSep,
		},
		{
			Name:    "R2.4_comma_sep_with_field_selection",
			Args:    []string{"-t", ",", "-1", "2", "-2", "1", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirCommaField,
		},

		// R3.1: -a FILENUM prints unpairable lines from the specified file.
		{
			Name:    "R3.1_unpair_file1",
			Args:    []string{"-a", "1", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirUnpair,
		},
		{
			Name:    "R3.1_unpair_file2",
			Args:    []string{"-a", "2", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirUnpair,
		},
		{
			Name:    "R3.1_unpair_both",
			Args:    []string{"-a", "1", "-a", "2", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirUnpair,
		},
		{
			Name:    "R3.1_unpair_all_match_no_extra",
			Args:    []string{"-a", "1", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirAllMatch,
		},

		// R3.2: -v FILENUM prints only unpairable lines, suppressing paired.
		{
			Name:    "R3.2_v_file1",
			Args:    []string{"-v", "1", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirVOnly,
		},
		{
			Name:    "R3.2_v_file2",
			Args:    []string{"-v", "2", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirVOnly,
		},
		{
			Name:    "R3.2_v_no_unpairable",
			Args:    []string{"-v", "1", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirAllMatch,
		},

		// R3.3: -e STRING replaces missing fields.
		{
			Name:    "R3.3_empty_replacement_with_format",
			Args:    []string{"-a", "1", "-e", "EMPTY", "-o", "0,1.2,2.2", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirEmptyRepl,
		},
		{
			Name:    "R3.3_empty_replacement_both_files",
			Args:    []string{"-a", "1", "-a", "2", "-e", "N/A", "-o", "0,1.2,2.2", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirUnpair,
		},

		// R3.4: --header treats first line as header.
		{
			Name:    "R3.4_header_mode",
			Args:    []string{"--header", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirHeader,
		},
		{
			Name:    "R3.4_header_unsorted_header_line",
			Args:    []string{"--header", "file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirHeaderUnsorted,
		},

		// Error: missing file exits 1.
		{
			Name:      "error_missing_file",
			Args:      []string{"nonexistent.txt", "file2.txt"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   dirAllMatch,
			ExitCode:  1,
			Normalize: errNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
