// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/paste against gpaste reference binary.
// Implements: prd027-paste R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3, R4.1, R4.2, R4.3, R4.4
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
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	// Create temp files for test cases.
	dir := t.TempDir()

	// file1: a\nb\n
	file1 := filepath.Join(dir, "file1.txt")
	if err := os.WriteFile(file1, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// file2: 1\n2\n
	file2 := filepath.Join(dir, "file2.txt")
	if err := os.WriteFile(file2, []byte("1\n2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// file3: x\ny\nz\n (longer than file1/file2)
	file3 := filepath.Join(dir, "file3.txt")
	if err := os.WriteFile(file3, []byte("x\ny\nz\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// file4: single line
	file4 := filepath.Join(dir, "file4.txt")
	if err := os.WriteFile(file4, []byte("only\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// empty file
	emptyFile := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// serial file: a\nb\nc\n
	serialFile := filepath.Join(dir, "serial.txt")
	if err := os.WriteFile(serialFile, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Two files with tab delimiter.
		{
			Name: "two files default delimiter",
			Args: []string{file1, file2},
		},
		// R1.2: Unequal length files — shorter contributes empty fields.
		{
			Name: "unequal length files",
			Args: []string{file1, file3},
		},
		// R1.2: Three files with different lengths.
		{
			Name: "three files different lengths",
			Args: []string{file4, file1, file3},
		},
		// R1.3: stdin via "-" designator.
		{
			Name: "stdin as dash",
			Args:  []string{"-", file2},
			Stdin: []byte("from_stdin\nline2\n"),
		},
		// R1.4: No files reads from stdin (passthrough).
		{
			Name:  "no files reads stdin",
			Args:  []string{},
			Stdin: []byte("hello\nworld\n"),
		},
		// R1.4: Single dash reads stdin.
		{
			Name:  "single dash reads stdin",
			Args:  []string{"-"},
			Stdin: []byte("one\ntwo\n"),
		},
		// R1.2: File with empty file.
		{
			Name: "file with empty file",
			Args: []string{file1, emptyFile},
		},
		// R1.2: Empty file with file.
		{
			Name: "empty file with file",
			Args: []string{emptyFile, file1},
		},
		// R1.3: Multiple dash arguments each consume next stdin line.
		{
			Name:  "multiple dash args",
			Args:  []string{"-", "-"},
			Stdin: []byte("line1\nline2\nline3\nline4\n"),
		},
		// R2.1: Single-character custom delimiter.
		{
			Name: "custom delimiter colon",
			Args: []string{"-d:", file1, file2},
		},
		// R2.1: Delimiter list cycles across fields.
		{
			Name: "delimiter list cycles across fields",
			Args: []string{"-d:;", file4, file1, file3},
		},
		// R2.1: Single delimiter with three files.
		{
			Name: "single delimiter three files",
			Args: []string{"-d,", file1, file2, file3},
		},
		// R2.2: Escape sequence \t (tab) in delimiter.
		{
			Name: "escape tab delimiter",
			Args: []string{`-d\t`, file1, file2},
		},
		// R2.2: Escape sequence \n (newline) in delimiter.
		{
			Name: "escape newline delimiter",
			Args: []string{`-d\n`, file1, file2},
		},
		// R2.2: Escape sequence \\ (backslash) in delimiter.
		{
			Name: "escape backslash delimiter",
			Args: []string{`-d\\`, file1, file2},
		},
		// R2.2: Escape sequence \0 (empty string) in delimiter.
		{
			Name: "escape zero delimiter",
			Args: []string{`-d\0`, file1, file2},
		},
		// R2.2: Mixed escape sequences in delimiter list.
		{
			Name: "mixed escape delimiters",
			Args: []string{`-d\t\0`, file4, file1, file3},
		},
		// R2.3: Delimiter cycling resets per output line.
		{
			Name: "delimiter cycling resets per line",
			Args: []string{"-d:;", file1, file2, file3},
		},
		// R2.1: Long option --delimiters with custom delimiter.
		{
			Name: "long option delimiters",
			Args: []string{"--delimiters=:", file1, file2},
		},
		// R2.1: -d with space-separated argument.
		{
			Name: "d flag space separated",
			Args: []string{"-d", ",", file1, file2},
		},
		// R2.2: \0 in delimiter list with other delimiters.
		{
			Name: "zero in delimiter list",
			Args: []string{`-d:\0;`, file4, file1, file2, file3},
		},
		// R3.1: Serial mode — all lines of one file joined on one output line.
		{
			Name: "serial single file",
			Args: []string{"-s", serialFile},
		},
		// R3.1: Serial mode with two files produces two output lines.
		{
			Name: "serial two files",
			Args: []string{"-s", file1, file2},
		},
		// R3.1: Serial mode with three files of different lengths.
		{
			Name: "serial three files different lengths",
			Args: []string{"-s", file4, file1, file3},
		},
		// R3.1: Serial mode with empty file produces empty output line.
		{
			Name: "serial empty file",
			Args: []string{"-s", emptyFile},
		},
		// R3.1: Serial mode with empty and non-empty files.
		{
			Name: "serial empty and non-empty",
			Args: []string{"-s", emptyFile, file1},
		},
		// R3.1: Serial mode with stdin via dash.
		{
			Name:  "serial stdin dash",
			Args:  []string{"-s", "-"},
			Stdin: []byte("x\ny\nz\n"),
		},
		// R3.2: Serial mode with custom delimiter cycles across fields.
		{
			Name: "serial custom delimiter",
			Args: []string{"-s", "-d:", serialFile},
		},
		// R3.2: Serial mode with delimiter list cycling.
		{
			Name: "serial delimiter list cycling",
			Args: []string{"-s", "-d:;", serialFile},
		},
		// R3.2: Serial mode delimiter cycling with multiple files.
		{
			Name: "serial delimiter cycling multiple files",
			Args: []string{"-s", "-d:;", file1, file3},
		},
		// R3.2: Serial mode with escape delimiter.
		{
			Name: "serial escape delimiter",
			Args: []string{"-s", `-d\n`, serialFile},
		},
		// R3.2: Serial mode with \0 delimiter.
		{
			Name: "serial zero delimiter",
			Args: []string{"-s", `-d\0`, serialFile},
		},
		// R3.3: -s overrides parallel mode — long option form.
		{
			Name: "serial long option",
			Args: []string{"--serial", serialFile},
		},
		// R3.3: -s combined with -d flag.
		{
			Name: "serial with delimiter flag",
			Args: []string{"-sd,", serialFile},
		},
		// R3.1: Serial mode single line file.
		{
			Name: "serial single line file",
			Args: []string{"-s", file4},
		},
		// R4.1: Exit 0 when all inputs processed successfully.
		{
			Name: "exit 0 on success",
			Args: []string{file1, file2},
		},
		// R4.1: Exit 0 with serial mode success.
		{
			Name: "exit 0 serial success",
			Args: []string{"-s", file1},
		},
		// R4.2: Exit 1 when input file cannot be opened.
		{
			Name:      "exit 1 nonexistent file",
			Args:      []string{filepath.Join(dir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R4.2: Exit 1 on nonexistent file in parallel with valid file.
		{
			Name:      "exit 1 nonexistent with valid file",
			Args:      []string{filepath.Join(dir, "no_such_file.txt"), file1},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R4.2: Exit 1 on nonexistent file in serial mode.
		{
			Name:      "exit 1 nonexistent serial",
			Args:      []string{"-s", filepath.Join(dir, "missing.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// stderrPattern matches error messages from both Go and gpaste to normalize them.
var stderrPattern = regexp.MustCompile(`(?m)^.*: .*: [Nn]o such file or directory\n?`)

// normalizeStderr replaces error messages with a canonical form so that
// differences in program name and path formatting between Go and gpaste
// do not cause false failures.
func normalizeStderr(b []byte) []byte {
	return stderrPattern.ReplaceAll(b, []byte("OPEN_ERROR\n"))
}
