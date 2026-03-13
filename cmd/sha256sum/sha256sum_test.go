// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sha256sum.
//
// Implements: prd032-sha256sum R1.1–R1.4, R2.1–R2.3, R3.1–R3.2, R4.1–R4.3
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

const binGsha256sum = "gsha256sum"

// sha256sumErrRe matches sha256sum/gsha256sum error lines and normalizes the program
// name and error format differences between GNU and Go implementations.
var sha256sumErrRe = regexp.MustCompile(`(?m)^g?sha256sum: .+?: .+$`)

// normalizeSha256sumErrors replaces sha256sum error lines with a canonical form so
// that minor wording differences between GNU and Go do not cause false failures.
func normalizeSha256sumErrors(b []byte) []byte {
	return sha256sumErrRe.ReplaceAll(b, []byte("PROG: FILE: ERROR"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGsha256sum)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGsha256sum, err)
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
		// R1.1: compute SHA-256 of a single file in text mode (default).
		{
			Name: "r1.1_single_file_text_mode",
			Args: []string{helloFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: compute SHA-256 of multiple files.
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
			Normalize: []testutils.NormalizeFunc{normalizeSha256sumErrors},
		},
		// R1.4: missing file then existing file — exit 1, continues processing.
		{
			Name:      "r1.4_missing_then_existing",
			Args:      []string{missing, helloFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSha256sumErrors},
		},
		// R1.4: existing file then missing file — exit 1.
		{
			Name:      "r1.4_existing_then_missing",
			Args:      []string{helloFile, missing},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSha256sumErrors},
		},
		// R1.4: permission denied — exit 1.
		{
			Name:      "r1.4_permission_denied",
			Args:      []string{unreadable},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSha256sumErrors},
		},
		// R1.4: multiple missing files — all errors, exit 1.
		{
			Name:      "r1.4_multiple_missing",
			Args:      []string{missing, filepath.Join(dir, "also_missing.txt")},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSha256sumErrors},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffBinaryTextMode runs differential tests for binary/text mode flags.
func TestDiffBinaryTextMode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGsha256sum)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGsha256sum, err)
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
		// -b produces "HASH *FILENAME".
		{
			Name: "binary_flag_short",
			Args: []string{"-b", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// --binary long form.
		{
			Name: "binary_flag_long",
			Args: []string{"--binary", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// -b with multiple files.
		{
			Name: "binary_multiple_files",
			Args: []string{"-b", dataFile, secondFile},
			Env:  []string{"LC_ALL=C"},
		},
		// -b with stdin.
		{
			Name:  "binary_stdin",
			Args:  []string{"-b"},
			Stdin: []byte("stdin binary\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// -t (text mode, default) produces "HASH  FILENAME" (two spaces).
		{
			Name: "text_flag_short",
			Args: []string{"-t", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// --text long form.
		{
			Name: "text_flag_long",
			Args: []string{"--text", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// Default (no flag) is text mode.
		{
			Name: "default_text_mode",
			Args: []string{dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// --tag with -b uses BSD tag format (mode has no effect).
		{
			Name: "tag_with_binary",
			Args: []string{"--tag", "-b", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// --tag without -b also uses BSD tag format.
		{
			Name: "tag_without_binary",
			Args: []string{"--tag", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// --tag with multiple files.
		{
			Name: "tag_multiple_files",
			Args: []string{"--tag", dataFile, secondFile},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCheck runs differential tests for check mode (R3.1–R3.2).
func TestDiffCheck(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGsha256sum)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGsha256sum, err)
	}

	dir := t.TempDir()

	// Create a test file with known content.
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("writing test.txt: %v", err)
	}

	// hello\n => 5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03
	correctHash := "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"

	// R3.1: valid checksum file in text mode format (two spaces).
	validCheckFile := filepath.Join(dir, "valid.sha256")
	if err := os.WriteFile(validCheckFile, []byte(correctHash+"  "+testFile+"\n"), 0o644); err != nil {
		t.Fatalf("writing valid.sha256: %v", err)
	}

	// R3.1: valid checksum file in binary mode format (space + asterisk).
	binaryCheckFile := filepath.Join(dir, "binary.sha256")
	if err := os.WriteFile(binaryCheckFile, []byte(correctHash+" *"+testFile+"\n"), 0o644); err != nil {
		t.Fatalf("writing binary.sha256: %v", err)
	}

	// R3.1: valid checksum in BSD tag format.
	bsdCheckFile := filepath.Join(dir, "bsd.sha256")
	if err := os.WriteFile(bsdCheckFile, []byte("SHA256 ("+testFile+") = "+correctHash+"\n"), 0o644); err != nil {
		t.Fatalf("writing bsd.sha256: %v", err)
	}

	// R3.2: checksum file with a wrong hash (mismatch).
	badHash := "0000000000000000000000000000000000000000000000000000000000000000"
	failCheckFile := filepath.Join(dir, "fail.sha256")
	if err := os.WriteFile(failCheckFile, []byte(badHash+"  "+testFile+"\n"), 0o644); err != nil {
		t.Fatalf("writing fail.sha256: %v", err)
	}

	// R3.2: mixed valid and invalid checksums.
	secondFile := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(secondFile, []byte("world\n"), 0o644); err != nil {
		t.Fatalf("writing second.txt: %v", err)
	}
	mixedCheckFile := filepath.Join(dir, "mixed.sha256")
	mixedContent := correctHash + "  " + testFile + "\n" + badHash + "  " + secondFile + "\n"
	if err := os.WriteFile(mixedCheckFile, []byte(mixedContent), 0o644); err != nil {
		t.Fatalf("writing mixed.sha256: %v", err)
	}

	// Checksum file with malformed lines.
	malformedCheckFile := filepath.Join(dir, "malformed.sha256")
	malformedContent := "this is not a valid checksum line\n" + correctHash + "  " + testFile + "\n"
	if err := os.WriteFile(malformedCheckFile, []byte(malformedContent), 0o644); err != nil {
		t.Fatalf("writing malformed.sha256: %v", err)
	}

	// All-malformed checksum file.
	allMalformedFile := filepath.Join(dir, "all_malformed.sha256")
	if err := os.WriteFile(allMalformedFile, []byte("garbage line 1\ngarbage line 2\n"), 0o644); err != nil {
		t.Fatalf("writing all_malformed.sha256: %v", err)
	}

	// Checksum file referencing a non-existent file.
	missingRefFile := filepath.Join(dir, "missing_ref.sha256")
	missingPath := filepath.Join(dir, "no_such_file.txt")
	if err := os.WriteFile(missingRefFile, []byte(correctHash+"  "+missingPath+"\n"), 0o644); err != nil {
		t.Fatalf("writing missing_ref.sha256: %v", err)
	}

	// normalizeCheckErrors normalizes error messages between GNU and Go for check mode.
	normalizeCheckErrors := func(b []byte) []byte {
		re := regexp.MustCompile(`g?sha256sum`)
		b = re.ReplaceAll(b, []byte("sha256sum"))
		b = []byte(strings.ReplaceAll(string(b), "No such file or directory", "no such file or directory"))
		return b
	}

	tests := []testutils.DiffTest{
		// R3.1: valid checksum file — all OK, exit 0.
		{
			Name: "r3.1_check_valid_text_mode",
			Args: []string{"-c", validCheckFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: valid checksum in binary format.
		{
			Name: "r3.1_check_valid_binary_mode",
			Args: []string{"-c", binaryCheckFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: valid checksum in BSD tag format.
		{
			Name: "r3.1_check_valid_bsd_tag",
			Args: []string{"-c", bsdCheckFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: mismatch — FAILED, exit 1.
		{
			Name:      "r3.2_check_mismatch",
			Args:      []string{"-c", failCheckFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// R3.2: mixed match and mismatch — exit 1.
		{
			Name:      "r3.2_check_mixed",
			Args:      []string{"-c", mixedCheckFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// Malformed lines with --warn — valid checksums pass so exit 0.
		{
			Name:      "check_malformed_with_warn",
			Args:      []string{"-c", "--warn", malformedCheckFile},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// Malformed lines without --warn — valid checksums pass so exit 0.
		{
			Name:      "check_malformed_no_warn",
			Args:      []string{"-c", malformedCheckFile},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// All malformed — exit 1.
		{
			Name:      "check_all_malformed",
			Args:      []string{"-c", allMalformedFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// --strict with malformed lines — exit 1.
		{
			Name:      "check_strict_malformed",
			Args:      []string{"-c", "--strict", malformedCheckFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// --strict with no malformed lines — exit 0.
		{
			Name: "check_strict_no_malformed",
			Args: []string{"-c", "--strict", validCheckFile},
			Env:  []string{"LC_ALL=C"},
		},
		// --strict --warn with malformed lines — exit 1 with per-line warnings.
		{
			Name:      "check_strict_warn_malformed",
			Args:      []string{"-c", "--strict", "--warn", malformedCheckFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// R3.1: check with stdin (pipe checksum lines via stdin).
		{
			Name:  "r3.1_check_stdin",
			Args:  []string{"-c"},
			Stdin: []byte(correctHash + "  " + testFile + "\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: missing file referenced in checksum — exit 1.
		{
			Name:      "r3.2_check_missing_file_ref",
			Args:      []string{"-c", missingRefFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// --quiet suppresses OK lines.
		{
			Name: "check_quiet_all_ok",
			Args: []string{"-c", "--quiet", validCheckFile},
			Env:  []string{"LC_ALL=C"},
		},
		// --quiet with failure still shows FAILED.
		{
			Name:      "check_quiet_with_failure",
			Args:      []string{"-c", "--quiet", failCheckFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// --status suppresses all output.
		{
			Name:      "check_status_all_ok",
			Args:      []string{"-c", "--status", validCheckFile},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// --status with failure — no output, exit 1.
		{
			Name:      "check_status_with_failure",
			Args:      []string{"-c", "--status", failCheckFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// --status with malformed lines — no output, exit 0 (valid lines pass).
		{
			Name:      "check_status_malformed",
			Args:      []string{"-c", "--status", malformedCheckFile},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// --status with all malformed — no output, exit 1.
		{
			Name:      "check_status_all_malformed",
			Args:      []string{"-c", "--status", allMalformedFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// --quiet with --strict and malformed — exit 1, suppresses OK.
		{
			Name:      "check_quiet_strict_malformed",
			Args:      []string{"-c", "--quiet", "--strict", malformedCheckFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// --status with --strict and malformed — no output, exit 1.
		{
			Name:      "check_status_strict_malformed",
			Args:      []string{"-c", "--status", "--strict", malformedCheckFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// --quiet with --warn and malformed — suppresses OK, shows warnings.
		{
			Name:      "check_quiet_warn_malformed",
			Args:      []string{"-c", "--quiet", "--warn", malformedCheckFile},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// --check with --warn on all-malformed file — exit 1.
		{
			Name:      "check_warn_all_malformed",
			Args:      []string{"-c", "--warn", allMalformedFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCheckErrors},
		},
		// R3.1: --check with long form flag.
		{
			Name: "check_long_form",
			Args: []string{"--check", validCheckFile},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffExitCodes runs differential tests for exit code and SIGPIPE behavior (R4.1–R4.3).
func TestDiffExitCodes(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGsha256sum)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGsha256sum, err)
	}

	dir := t.TempDir()

	// Known-content file.
	goodFile := filepath.Join(dir, "good.txt")
	if err := os.WriteFile(goodFile, []byte("exit code test\n"), 0o644); err != nil {
		t.Fatalf("writing good.txt: %v", err)
	}

	secondGood := filepath.Join(dir, "good2.txt")
	if err := os.WriteFile(secondGood, []byte("second\n"), 0o644); err != nil {
		t.Fatalf("writing good2.txt: %v", err)
	}

	// Correct SHA-256 hash for "exit code test\n".
	// Computed dynamically by the differential test — we just need a valid checksum file.
	// Use gsha256sum to get the hash, but for the test we generate it inline.
	// echo -n "exit code test\n" | sha256sum => we let the diff test validate correctness.
	// For the checksum file, compute it from the reference binary at test time.
	correctHash := computeTestHash(t, goodFile)

	// Valid checksum file for --check exit 0.
	validCheck := filepath.Join(dir, "valid_exit.sha256")
	if err := os.WriteFile(validCheck, []byte(correctHash+"  "+goodFile+"\n"), 0o644); err != nil {
		t.Fatalf("writing valid_exit.sha256: %v", err)
	}

	// Wrong hash for --check exit 1.
	badHash := "0000000000000000000000000000000000000000000000000000000000000000"
	failCheck := filepath.Join(dir, "fail_exit.sha256")
	if err := os.WriteFile(failCheck, []byte(badHash+"  "+goodFile+"\n"), 0o644); err != nil {
		t.Fatalf("writing fail_exit.sha256: %v", err)
	}

	// Checksum file referencing non-existent file.
	missingRef := filepath.Join(dir, "missing_exit.sha256")
	missingPath := filepath.Join(dir, "no_such.txt")
	if err := os.WriteFile(missingRef, []byte(correctHash+"  "+missingPath+"\n"), 0o644); err != nil {
		t.Fatalf("writing missing_exit.sha256: %v", err)
	}

	// Non-existent checksum file itself.
	nonexistentCheck := filepath.Join(dir, "no_such_check.sha256")

	// normalizeExitErrors normalizes error messages between GNU and Go.
	normalizeExitErrors := func(b []byte) []byte {
		re := regexp.MustCompile(`g?sha256sum`)
		b = re.ReplaceAll(b, []byte("sha256sum"))
		b = []byte(strings.ReplaceAll(string(b), "No such file or directory", "no such file or directory"))
		return b
	}

	tests := []testutils.DiffTest{
		// R4.1: exit 0 when single file processed successfully.
		{
			Name: "r4.1_exit_0_single_file",
			Args: []string{goodFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.1: exit 0 when multiple files processed successfully.
		{
			Name: "r4.1_exit_0_multiple_files",
			Args: []string{goodFile, secondGood},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.1: exit 0 when stdin processed successfully.
		{
			Name:  "r4.1_exit_0_stdin",
			Stdin: []byte("stdin ok\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.1: exit 0 when --check with all digests matching.
		{
			Name: "r4.1_exit_0_check_all_match",
			Args: []string{"-c", validCheck},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.2: exit 1 when file cannot be opened.
		{
			Name:      "r4.2_exit_1_missing_file",
			Args:      []string{filepath.Join(dir, "nonexistent.txt")},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeExitErrors},
		},
		// R4.2: exit 1 when digest fails verification in --check.
		{
			Name:      "r4.2_exit_1_check_mismatch",
			Args:      []string{"-c", failCheck},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeExitErrors},
		},
		// R4.2: exit 1 when checked file does not exist.
		{
			Name:      "r4.2_exit_1_check_missing_ref",
			Args:      []string{"-c", missingRef},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeExitErrors},
		},
		// R4.2: exit 1 when checksum file itself is unreadable.
		{
			Name:      "r4.2_exit_1_unreadable_checksum_file",
			Args:      []string{"-c", nonexistentCheck},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeExitErrors},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// computeTestHash computes the SHA-256 hash of a file for use in test fixtures.
func computeTestHash(t *testing.T, path string) string {
	t.Helper()
	hash, err := computeFileHash(path)
	if err != nil {
		t.Fatalf("computing hash for %s: %v", path, err)
	}
	return hash
}
