// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/md5sum.
//
// Implements: prd030-md5sum R1.1–R1.4, R2.1–R2.4, R3.1–R3.3
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const binGmd5sum = "gmd5sum"

// md5sumErrRe matches md5sum/gmd5sum error lines and normalizes the program
// name and error format differences between GNU and Go implementations.
var md5sumErrRe = regexp.MustCompile(`(?m)^g?md5sum: .+?: .+$`)

// normalizeMd5sumErrors replaces md5sum error lines with a canonical form so
// that minor wording differences between GNU and Go do not cause false failures.
func normalizeMd5sumErrors(b []byte) []byte {
	return md5sumErrRe.ReplaceAll(b, []byte("PROG: FILE: ERROR"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGmd5sum)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGmd5sum, err)
	}

	// Create temp files for file-based test cases.
	dir := t.TempDir()

	// Known-content file for digest tests.
	helloFile := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(helloFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("writing hello.txt: %v", err)
	}

	// Second file for multi-file tests.
	worldFile := filepath.Join(dir, "world.txt")
	if err := os.WriteFile(worldFile, []byte("world\n"), 0o644); err != nil {
		t.Fatalf("writing world.txt: %v", err)
	}

	// Empty file.
	emptyFile := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatalf("writing empty.txt: %v", err)
	}

	// Non-existent file for error tests.
	missing := filepath.Join(dir, "nonexistent.txt")

	// Unreadable file for permission-denied tests.
	unreadable := filepath.Join(dir, "noperm.txt")
	if err := os.WriteFile(unreadable, []byte("secret\n"), 0o000); err != nil {
		t.Fatalf("writing noperm.txt: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: compute MD5 of a single file in text mode (default).
		{
			Name: "r1.1_single_file_text_mode",
			Args: []string{helloFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: compute MD5 of multiple files.
		{
			Name: "r1.1_multiple_files",
			Args: []string{helloFile, worldFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: binary mode output format "HASH *FILENAME".
		{
			Name: "r1.1_binary_mode",
			Args: []string{"-b", helloFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: empty file produces valid digest.
		{
			Name: "r1.1_empty_file",
			Args: []string{emptyFile},
			Env:  []string{"LC_ALL=C"},
		},

		// R1.2: stdin when no file arguments.
		{
			Name:  "r1.2_stdin_no_args",
			Stdin: []byte("abc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: stdin with empty input.
		{
			Name:  "r1.2_stdin_empty",
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: "-" means stdin.
		{
			Name:  "r1.2_dash_means_stdin",
			Args:  []string{"-"},
			Stdin: []byte("test input\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: stdin with newline content.
		{
			Name:  "r1.2_stdin_with_newline",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R1.3: --tag BSD-style output.
		{
			Name: "r1.3_tag_mode_file",
			Args: []string{"--tag", helloFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: --tag with stdin.
		{
			Name:  "r1.3_tag_mode_stdin",
			Args:  []string{"--tag"},
			Stdin: []byte("abc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: --tag with multiple files.
		{
			Name: "r1.3_tag_mode_multiple_files",
			Args: []string{"--tag", helloFile, worldFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: --tag with -b (binary flag has no effect on tag format).
		{
			Name: "r1.3_tag_with_binary",
			Args: []string{"--tag", "-b", helloFile},
			Env:  []string{"LC_ALL=C"},
		},

		// R1.4: exit 1 on missing file.
		{
			Name:      "r1.4_missing_file",
			Args:      []string{missing},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeMd5sumErrors},
		},
		// R1.4: missing file then existing file — exit 1, continues processing.
		{
			Name:      "r1.4_missing_then_existing",
			Args:      []string{missing, helloFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeMd5sumErrors},
		},
		// R1.4: existing file then missing file — exit 1.
		{
			Name:      "r1.4_existing_then_missing",
			Args:      []string{helloFile, missing},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeMd5sumErrors},
		},
		// R1.4: permission denied — exit 1.
		{
			Name:      "r1.4_permission_denied",
			Args:      []string{unreadable},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeMd5sumErrors},
		},
		// R1.4: multiple missing files — all errors, exit 1.
		{
			Name:      "r1.4_multiple_missing",
			Args:      []string{missing, filepath.Join(dir, "also_missing.txt")},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeMd5sumErrors},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffBinaryTextMode runs differential tests for binary/text mode flags (R3.1–R3.3).
func TestDiffBinaryTextMode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGmd5sum)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGmd5sum, err)
	}

	dir := t.TempDir()

	dataFile := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(dataFile, []byte("binary text mode test\n"), 0o644); err != nil {
		t.Fatalf("writing data.txt: %v", err)
	}

	secondFile := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(secondFile, []byte("second\n"), 0o644); err != nil {
		t.Fatalf("writing second.txt: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.1: -b produces "HASH *FILENAME".
		{
			Name: "r3.1_binary_flag_short",
			Args: []string{"-b", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: --binary long form.
		{
			Name: "r3.1_binary_flag_long",
			Args: []string{"--binary", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: -b with multiple files.
		{
			Name: "r3.1_binary_multiple_files",
			Args: []string{"-b", dataFile, secondFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: -b with stdin.
		{
			Name:  "r3.1_binary_stdin",
			Args:  []string{"-b"},
			Stdin: []byte("stdin binary\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: -t (text mode, default) produces "HASH  FILENAME" (two spaces).
		{
			Name: "r3.2_text_flag_short",
			Args: []string{"-t", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: --text long form.
		{
			Name: "r3.2_text_flag_long",
			Args: []string{"--text", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: default (no flag) is text mode.
		{
			Name: "r3.2_default_text_mode",
			Args: []string{dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --tag with -b uses BSD tag format (mode has no effect).
		{
			Name: "r3.3_tag_with_binary",
			Args: []string{"--tag", "-b", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --tag without -b also uses BSD tag format.
		{
			Name: "r3.3_tag_without_binary",
			Args: []string{"--tag", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --tag with multiple files.
		{
			Name: "r3.3_tag_multiple_files",
			Args: []string{"--tag", dataFile, secondFile},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCheck runs differential tests for check mode (R2.1–R2.4).
func TestDiffCheck(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGmd5sum)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGmd5sum, err)
	}

	dir := t.TempDir()

	// Create a test file with known content.
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("writing test.txt: %v", err)
	}

	// Compute correct hash via the reference binary to build checksum files.
	// hello\n => b1946ac92492d2347c6235b4d2611184
	correctHash := "b1946ac92492d2347c6235b4d2611184"

	// R2.1: valid checksum file in text mode format (two spaces).
	validCheckFile := filepath.Join(dir, "valid.md5")
	if err := os.WriteFile(validCheckFile, []byte(correctHash+"  "+testFile+"\n"), 0o644); err != nil {
		t.Fatalf("writing valid.md5: %v", err)
	}

	// R2.1: valid checksum file in binary mode format (space + asterisk).
	binaryCheckFile := filepath.Join(dir, "binary.md5")
	if err := os.WriteFile(binaryCheckFile, []byte(correctHash+" *"+testFile+"\n"), 0o644); err != nil {
		t.Fatalf("writing binary.md5: %v", err)
	}

	// R2.1: valid checksum in BSD tag format.
	bsdCheckFile := filepath.Join(dir, "bsd.md5")
	if err := os.WriteFile(bsdCheckFile, []byte("MD5 ("+testFile+") = "+correctHash+"\n"), 0o644); err != nil {
		t.Fatalf("writing bsd.md5: %v", err)
	}

	// R2.2: checksum file with a wrong hash (mismatch).
	badHash := "0000000000000000000000000000dead"
	failCheckFile := filepath.Join(dir, "fail.md5")
	if err := os.WriteFile(failCheckFile, []byte(badHash+"  "+testFile+"\n"), 0o644); err != nil {
		t.Fatalf("writing fail.md5: %v", err)
	}

	// R2.2: mixed valid and invalid checksums.
	secondFile := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(secondFile, []byte("world\n"), 0o644); err != nil {
		t.Fatalf("writing second.txt: %v", err)
	}
	// world\n => c0e6549d15ec25f21a80e1f979cd1ff4 -- but use a bad hash for it
	mixedCheckFile := filepath.Join(dir, "mixed.md5")
	mixedContent := correctHash + "  " + testFile + "\n" + badHash + "  " + secondFile + "\n"
	if err := os.WriteFile(mixedCheckFile, []byte(mixedContent), 0o644); err != nil {
		t.Fatalf("writing mixed.md5: %v", err)
	}

	// R2.3: checksum file with malformed lines.
	malformedCheckFile := filepath.Join(dir, "malformed.md5")
	malformedContent := "this is not a valid checksum line\n" + correctHash + "  " + testFile + "\n"
	if err := os.WriteFile(malformedCheckFile, []byte(malformedContent), 0o644); err != nil {
		t.Fatalf("writing malformed.md5: %v", err)
	}

	// All-malformed checksum file.
	allMalformedFile := filepath.Join(dir, "all_malformed.md5")
	if err := os.WriteFile(allMalformedFile, []byte("garbage line 1\ngarbage line 2\n"), 0o644); err != nil {
		t.Fatalf("writing all_malformed.md5: %v", err)
	}

	// Checksum file referencing a non-existent file.
	missingRefFile := filepath.Join(dir, "missing_ref.md5")
	missingPath := filepath.Join(dir, "no_such_file.txt")
	if err := os.WriteFile(missingRefFile, []byte(correctHash+"  "+missingPath+"\n"), 0o644); err != nil {
		t.Fatalf("writing missing_ref.md5: %v", err)
	}

	// normalizeCheckErrors normalizes error messages between GNU and Go for check mode.
	normalizeCheckErrors := func(b []byte) []byte {
		// Normalize program name differences (gmd5sum vs md5sum).
		re := regexp.MustCompile(`g?md5sum`)
		b = re.ReplaceAll(b, []byte("md5sum"))
		// Normalize error message casing (Go: "no such file or directory", GNU: "No such file or directory").
		b = []byte(strings.ReplaceAll(string(b), "No such file or directory", "no such file or directory"))
		return b
	}

	tests := []testutils.DiffTest{
		// R2.1, R2.2: valid checksum file — all OK, exit 0.
		{
			Name: "r2.1_check_valid_text_mode",
			Args: []string{"-c", validCheckFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: valid checksum in binary format.
		{
			Name: "r2.1_check_valid_binary_mode",
			Args: []string{"-c", binaryCheckFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: valid checksum in BSD tag format.
		{
			Name: "r2.1_check_valid_bsd_tag",
			Args: []string{"-c", bsdCheckFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: mismatch — FAILED, exit 1.
		{
			Name:      "r2.2_check_mismatch",
			Args:      []string{"-c", failCheckFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// R2.2: mixed match and mismatch — exit 1.
		{
			Name:      "r2.2_check_mixed",
			Args:      []string{"-c", mixedCheckFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// R2.3: malformed lines with --warn — valid checksums pass so exit 0.
		{
			Name:      "r2.3_check_malformed_with_warn",
			Args:      []string{"-c", "--warn", malformedCheckFile},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// R2.3: malformed lines without --warn — valid checksums pass so exit 0.
		{
			Name:      "r2.3_check_malformed_no_warn",
			Args:      []string{"-c", malformedCheckFile},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// R2.3: all malformed — exit 1.
		{
			Name:      "r2.3_check_all_malformed",
			Args:      []string{"-c", allMalformedFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// R2.4: --quiet suppresses OK lines.
		{
			Name: "r2.4_check_quiet_all_ok",
			Args: []string{"-c", "--quiet", validCheckFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: --quiet with failure still shows FAILED.
		{
			Name:      "r2.4_check_quiet_with_failure",
			Args:      []string{"-c", "--quiet", failCheckFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// R2.4: --status suppresses all output.
		{
			Name:      "r2.4_check_status_all_ok",
			Args:      []string{"-c", "--status", validCheckFile},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// R2.4: --status with failure — no output, exit 1.
		{
			Name:      "r2.4_check_status_with_failure",
			Args:      []string{"-c", "--status", failCheckFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// R2.1: check with stdin (pipe checksum lines via stdin).
		{
			Name:  "r2.1_check_stdin",
			Args:  []string{"-c"},
			Stdin: []byte(correctHash + "  " + testFile + "\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: missing file referenced in checksum — exit 1.
		{
			Name:      "r2.2_check_missing_file_ref",
			Args:      []string{"-c", missingRefFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
