// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
	return p
}

// tacErrorNormalizer replaces tac error message lines with a fixed
// placeholder so different error formats between GNU and Go do not
// cause divergence. GNU uses "tac: cannot open 'F' for reading: MSG"
// while Go uses "tac: open F: msg".
var tacErrorNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	re := regexp.MustCompile(`(?m)^(g?tac|tac): .*$`)
	return re.ReplaceAll(data, []byte("tac: <FILE_ERROR>"))
}

// TestDiff runs differential tests comparing the Go tac binary against
// the GNU reference binary gtac.
//
// Implements prd021-tac R4.1, R4.2, R4.3.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("gtac")
	if err != nil {
		t.Skipf("reference binary gtac not in PATH: %v", err)
	}

	// Create temp files for file-argument tests (R1.4).
	tmpDir := t.TempDir()
	file1 := writeTestFile(t, tmpDir, "f1.txt", "alpha\nbeta\ngamma\n")
	file2 := writeTestFile(t, tmpDir, "f2.txt", "one\ntwo\n")

	// R3.2: path to a file that does not exist.
	nonexistent := filepath.Join(tmpDir, "no_such_file.txt")

	tests := []testutils.DiffTest{
		{
			// R1.1, R1.2: lines reversed; trailing newline preserved.
			Name:  "tac_default_reversal",
			Stdin: []byte("alpha\nbeta\ngamma\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: no trailing newline on input.
			Name:  "tac_no_trailing_newline",
			Stdin: []byte("a\nb\nc"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: single line reverses to itself.
			Name:  "tac_single_line",
			Stdin: []byte("only\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: empty input produces empty output.
			Name:  "tac_empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.3: explicit "-" reads from stdin.
			Name:  "tac_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("x\ny\nz\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.4: single file argument.
			Name: "tac_file_arg",
			Args: []string{file1},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R1.4: multiple files processed independently.
			Name: "tac_multi_file",
			Args: []string{file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R2.1: custom single-character separator.
			Name:  "tac_custom_sep",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c:"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1: custom multi-character separator.
			Name:  "tac_multichar_sep",
			Args:  []string{"-s", "::"},
			Stdin: []byte("a::b::c::"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.1: custom separator with no trailing separator.
			Name:  "tac_custom_sep_no_trailing",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.2: -b places separator before records.
			Name:  "tac_before_sep",
			Args:  []string{"-b", "-s", ":"},
			Stdin: []byte(":a:b:c"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.2: -b with trailing separator.
			Name:  "tac_before_sep_trailing",
			Args:  []string{"-b", "-s", ":"},
			Stdin: []byte(":a:b:c:"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.3, R2.4: -r interprets separator as regex.
			Name:  "tac_regex_sep",
			Args:  []string{"-r", "-s", "[:|]"},
			Stdin: []byte("a:b|c:"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.2, R2.3, R2.4: -b -r combined.
			Name:  "tac_before_regex_sep",
			Args:  []string{"-b", "-r", "-s", "[:|]"},
			Stdin: []byte(":a|b:c"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: exit 0 on successful processing.
		{
			Name:     "tac_exit_zero_on_success",
			Args:     []string{file1},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.2: exit 1 when a file cannot be opened.
		{
			Name:      "tac_nonexistent_file",
			Args:      []string{nonexistent},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{tacErrorNormalizer},
		},
		// R3.2: exit 1 with mixed valid and invalid files; valid files
		// are still processed and remaining files continue.
		{
			Name:      "tac_mixed_valid_invalid_files",
			Args:      []string{file1, nonexistent, file2},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{tacErrorNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
