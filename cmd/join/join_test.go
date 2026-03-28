// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/join against GNU gjoin.
// Covers prd069-join R4.1-R4.4 (differential testing).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// writeFile creates a file with the given content in dir and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", p, err)
	}
	return p
}

// stderrNormalizer normalizes error messages between GNU gjoin and Go join.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?join|gjoin`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	noSuch := regexp.MustCompile(`(?i)no such file or directory`)
	missingOp := regexp.MustCompile(
		`(?m)^join: missing operand after '[^']*'\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("join"))
		b = tryHelp.ReplaceAll(b, nil)
		b = noSuch.ReplaceAll(b, []byte("No such file or directory"))
		b = missingOp.ReplaceAll(b, []byte("join: missing operand\n"))
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gjoin")
	if err != nil {
		t.Skipf("reference binary gjoin not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	dir := t.TempDir()

	// Basic test files: sorted on first field.
	f1 := writeFile(t, dir, "f1.txt", "a 1\nb 2\nc 3\n")
	f2 := writeFile(t, dir, "f2.txt", "a X\nb Y\nc Z\n")

	// Partial overlap files.
	partial1 := writeFile(t, dir, "p1.txt", "a 1\nb 2\nd 4\n")
	partial2 := writeFile(t, dir, "p2.txt", "a X\nc W\nd Z\n")

	// CSV files sorted on different fields.
	csv1 := writeFile(t, dir, "csv1.txt", "1,a,foo\n2,b,bar\n3,c,baz\n")
	csv2 := writeFile(t, dir, "csv2.txt", "a,10\nb,20\nc,30\n")

	// Files for -j test (join on field 2).
	jf1 := writeFile(t, dir, "jf1.txt", "x a\ny b\nz c\n")
	jf2 := writeFile(t, dir, "jf2.txt", "p a\nq b\nr c\n")

	// Files for -o format test.
	of1 := writeFile(t, dir, "of1.txt", "a 1 alpha\nb 2 beta\n")
	of2 := writeFile(t, dir, "of2.txt", "a X one\nb Y two\n")

	// Files for unpairable tests.
	unp1 := writeFile(t, dir, "unp1.txt", "a 1\nb 2\nc 3\nd 4\n")
	unp2 := writeFile(t, dir, "unp2.txt", "b X\nd Y\n")

	// Files for -e empty replacement.
	ef1 := writeFile(t, dir, "ef1.txt", "a 1\nb 2\nc 3\n")
	ef2 := writeFile(t, dir, "ef2.txt", "a X\nc\n")

	// Files for case-insensitive join.
	ci1 := writeFile(t, dir, "ci1.txt", "A 1\nB 2\nC 3\n")
	ci2 := writeFile(t, dir, "ci2.txt", "a X\nb Y\nc Z\n")

	// Files for --header test.
	hdr1 := writeFile(t, dir, "hdr1.txt", "Name Score\na 10\nb 20\n")
	hdr2 := writeFile(t, dir, "hdr2.txt", "Name Grade\na A\nb B\n")

	// Empty file.
	emptyFile := writeFile(t, dir, "empty.txt", "")

	// Disjoint files.
	disj1 := writeFile(t, dir, "disj1.txt", "a 1\nc 3\ne 5\n")
	disj2 := writeFile(t, dir, "disj2.txt", "b 2\nd 4\nf 6\n")

	// Duplicate keys in file 2.
	dup1 := writeFile(t, dir, "dup1.txt", "a 1\nb 2\n")
	dup2 := writeFile(t, dir, "dup2.txt", "a X\na Y\nb Z\n")

	// Files for -v test.
	vf1 := writeFile(t, dir, "vf1.txt", "a 1\nb 2\nc 3\n")
	vf2 := writeFile(t, dir, "vf2.txt", "b X\n")

	tests := []testutils.DiffTest{
		// --- R4.1: Test scaffold, basic exit 0 ---

		// R1.1, R1.2: Default join on first field.
		{
			Name: "default_join",
			Args: []string{f1, f2},
		},

		// --- R4.2: Basic join operations ---

		// R1.3: Unpairable lines suppressed by default.
		{
			Name: "default_suppresses_unpairable",
			Args: []string{partial1, partial2},
		},

		// R2.4: Custom separator with -t.
		{
			Name: "custom_separator_comma",
			Args: []string{"-t", ",", "-1", "2", "-2", "1", csv1, csv2},
		},

		// R2.1: -1 and -2 field selection.
		{
			Name: "field_selection_1_2",
			Args: []string{"-1", "2", "-2", "1", jf1, jf2},
		},

		// R2.2: -j combined field.
		{
			Name: "joint_field_j",
			Args: []string{"-j", "2", jf1, jf2},
		},

		// R2.3: -o output format.
		{
			Name: "output_format_o",
			Args: []string{"-o", "0,1.2,1.3,2.2,2.3", of1, of2},
		},

		// R2.3: -o with just the join field.
		{
			Name: "output_format_o_joinfield_only",
			Args: []string{"-o", "0", f1, f2},
		},

		// R3.1: -a 1 prints unpairable from file 1.
		{
			Name: "unpairable_a1",
			Args: []string{"-a", "1", unp1, unp2},
		},

		// R3.1: -a 2 prints unpairable from file 2.
		{
			Name: "unpairable_a2",
			Args: []string{"-a", "2", partial1, partial2},
		},

		// R3.1: -a 1 -a 2 prints unpairable from both.
		{
			Name: "unpairable_a1_a2",
			Args: []string{"-a", "1", "-a", "2", partial1, partial2},
		},

		// R3.2: -v 1 prints only unpairable from file 1.
		{
			Name: "only_unpairable_v1",
			Args: []string{"-v", "1", vf1, vf2},
		},

		// R3.2: -v 2 prints only unpairable from file 2.
		{
			Name: "only_unpairable_v2",
			Args: []string{"-v", "2", partial1, partial2},
		},

		// Duplicate keys — cartesian product.
		{
			Name: "duplicate_keys_file2",
			Args: []string{dup1, dup2},
		},

		// Disjoint files — no output by default.
		{
			Name: "disjoint_no_output",
			Args: []string{disj1, disj2},
		},

		// Disjoint files with -a 1 -a 2.
		{
			Name: "disjoint_with_a1_a2",
			Args: []string{"-a", "1", "-a", "2", disj1, disj2},
		},

		// Empty file as file 1.
		{
			Name: "empty_file1",
			Args: []string{f1, emptyFile},
		},

		// Empty file as file 2.
		{
			Name: "empty_file2",
			Args: []string{emptyFile, f2},
		},

		// Both files empty.
		{
			Name: "both_empty",
			Args: []string{emptyFile, emptyFile},
		},

		// --- R4.3: Advanced features ---

		// R3.3: -e empty replacement for missing fields.
		{
			Name: "empty_replacement_e",
			Args: []string{"-a", "1", "-e", "EMPTY", "-o", "0,1.2,2.2", ef1, ef2},
		},

		// R3.3: -e with -v (only unpairable).
		{
			Name: "empty_replacement_with_v",
			Args: []string{"-v", "1", "-e", "NONE", "-o", "0,1.2,2.2", unp1, unp2},
		},

		// -i / --ignore-case for case-insensitive join.
		{
			Name: "ignore_case_i",
			Args: []string{"-i", ci1, ci2},
		},

		// --ignore-case long form.
		{
			Name: "ignore_case_long",
			Args: []string{"--ignore-case", ci1, ci2},
		},

		// R3.4: --header treats first lines as headers.
		{
			Name: "header_mode",
			Args: []string{"--header", hdr1, hdr2},
		},

		// --check-order on sorted input (no warning).
		{
			Name: "check_order_sorted",
			Args: []string{"--check-order", f1, f2},
		},


		// R1.4: stdin via '-' as file 1.
		{
			Name:  "stdin_as_file1",
			Args:  []string{"-", f2},
			Stdin: []byte("a 1\nb 2\nc 3\n"),
		},

		// R1.4: stdin via '-' as file 2.
		{
			Name:  "stdin_as_file2",
			Args:  []string{f1, "-"},
			Stdin: []byte("a X\nb Y\nc Z\n"),
		},

		// --- R4.4: Error conditions ---

		// Missing file operand.
		{
			Name:      "missing_operand",
			Args:      []string{},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},

		// Only one file operand.
		{
			Name:      "single_operand",
			Args:      []string{f1},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},

		// Nonexistent file.
		{
			Name:      "nonexistent_file",
			Args:      []string{"/nonexistent-path/no-such-file.txt", f2},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},

		// Invalid flag.
		{
			Name:      "invalid_flag",
			Args:      []string{"--badopt", f1, f2},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},

		// Combined test: -t, -a, -e, -o together.
		{
			Name: "combined_t_a_e_o",
			Args: []string{
				"-t", ",",
				"-a", "1",
				"-e", "N/A",
				"-o", "0,1.2,2.2",
				writeFile(t, dir, "combo1.txt", "a,1\nb,2\nc,3\n"),
				writeFile(t, dir, "combo2.txt", "a,X\nc,Z\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
