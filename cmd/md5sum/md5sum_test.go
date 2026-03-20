// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd030-md5sum R1.1, R1.2, R1.3, R1.4,
// R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// setupFileDir creates a temp directory with the given files and returns the path.
func setupFileDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
		if err != nil {
			t.Fatalf("writing test file %s: %v", name, err)
		}
	}
	return dir
}

// errLinePattern matches error lines from both gmd5sum and md5sum.
var errLinePattern = regexp.MustCompile(`(?:g?md5sum): [^\n]*\n`)

// stderrNormalizer replaces stderr error lines with a fixed placeholder
// to account for binary name and message format differences.
func stderrNormalizer(data []byte) []byte {
	return errLinePattern.ReplaceAll(data, []byte("md5sum: <ERROR>\n"))
}

// warnLinePattern matches warning lines about improperly formatted lines.
var warnLinePattern = regexp.MustCompile(`(?:g?md5sum): [^:]+: \d+: improperly formatted[^\n]*\n`)

// warnNormalizer normalizes warning lines about malformed checksum lines.
func warnNormalizer(data []byte) []byte {
	return warnLinePattern.ReplaceAll(data, []byte("md5sum: WARN_LINE\n"))
}

// failedSummaryPattern matches the summary line printed after --check.
var failedSummaryPattern = regexp.MustCompile(`(?:g?md5sum): WARNING: \d+ computed checksum[^\n]*\n`)

// failedSummaryNormalizer normalizes the summary warning line.
func failedSummaryNormalizer(data []byte) []byte {
	return failedSummaryPattern.ReplaceAll(data, []byte("md5sum: WARNING_SUMMARY\n"))
}

// md5hex computes md5 hex of a string for building checksum files.
func md5hex(content string) string {
	// Known MD5 hashes for test data
	switch content {
	case "hello world\n":
		return "6f5902ac237024bdd0c176cb93063dc4"
	case "test data\n":
		return "eb733a00c0c9d336e65691a37ab54293"
	default:
		return ""
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmd5sum")
	if err != nil {
		t.Skipf("reference binary gmd5sum not in PATH: %v", err)
	}

	dirSingle := setupFileDir(t, map[string]string{
		"input.txt": "hello world\n",
	})
	dirTag := setupFileDir(t, map[string]string{
		"input.txt": "test data\n",
	})
	dirMulti := setupFileDir(t, map[string]string{
		"a.txt": "aaa\n",
		"b.txt": "bbb\n",
	})
	dirMissing := setupFileDir(t, map[string]string{
		"exists.txt": "data\n",
	})

	// R2.1/R2.2: Set up a directory with a valid checksum file.
	dirCheck := setupCheckDir(t, "hello world\n")

	// R2.1/R2.2: Checksum file with a mismatch.
	dirCheckFail := setupCheckFailDir(t)

	// R2.3: Checksum file with malformed lines.
	dirCheckWarn := setupCheckWarnDir(t)

	// R2.4: --quiet and --status test directories.
	dirCheckQuiet := setupCheckDir(t, "hello world\n")
	dirCheckStatus := setupCheckDir(t, "hello world\n")
	dirCheckStatusFail := setupCheckFailDir(t)

	// R2.1: BSD tag format check file.
	dirCheckBSD := setupCheckBSDDir(t)

	tests := []testutils.DiffTest{
		// R1.1: Compute digest of a file in text mode (default).
		{
			Name:    "file_text_mode",
			Args:    []string{"input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.1: Compute digest of a file in binary mode.
		{
			Name:    "file_binary_mode",
			Args:    []string{"-b", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.2: Read from stdin when no file arguments given.
		{
			Name:  "stdin_no_args",
			Stdin: []byte("abc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: Read from stdin when "-" is given as filename.
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("abc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: BSD tag output format.
		{
			Name:    "tag_format",
			Args:    []string{"--tag", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirTag,
		},
		// R1.4: Error on nonexistent file, exit 1, continue remaining.
		{
			Name:      "missing_file_continues",
			Args:      []string{"nonexistent.txt", "exists.txt"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			WorkDir:   dirMissing,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R1.1: Multiple files.
		{
			Name:    "multiple_files",
			Args:    []string{"a.txt", "b.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirMulti,
		},
		// R1.2: Empty stdin.
		{
			Name:  "empty_stdin",
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1/R2.2: --check with valid checksum file, all pass.
		{
			Name:      "check_valid",
			Args:      []string{"--check", "checksums.txt"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   dirCheck,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R2.1/R2.2: --check with a mismatch, exit 1.
		{
			Name:      "check_failed",
			Args:      []string{"--check", "checksums.txt"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			WorkDir:   dirCheckFail,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer, failedSummaryNormalizer},
		},
		// R2.3: --check --warn with malformed lines.
		{
			Name:      "check_warn",
			Args:      []string{"--check", "--warn", "checksums.txt"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   dirCheckWarn,
			Normalize: []testutils.NormalizeFunc{warnNormalizer, stderrNormalizer, failedSummaryNormalizer},
		},
		// R2.4: --check --quiet suppresses OK lines.
		{
			Name:      "check_quiet",
			Args:      []string{"--check", "--quiet", "checksums.txt"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   dirCheckQuiet,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R2.4: --check --status suppresses all output.
		{
			Name:      "check_status_pass",
			Args:      []string{"--check", "--status", "checksums.txt"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   dirCheckStatus,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R2.4: --check --status with failure, exit 1, no output.
		{
			Name:      "check_status_fail",
			Args:      []string{"--check", "--status", "checksums.txt"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			WorkDir:   dirCheckStatusFail,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer, failedSummaryNormalizer},
		},
		// R2.1: --check with BSD tag format checksum file.
		{
			Name:      "check_bsd_format",
			Args:      []string{"--check", "checksums.txt"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   dirCheckBSD,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R3.1: -b flag produces "HASH *FILENAME" format.
		{
			Name:    "binary_flag_short",
			Args:    []string{"-b", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R3.1: --binary flag produces "HASH *FILENAME" format.
		{
			Name:    "binary_flag_long",
			Args:    []string{"--binary", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R3.1: -b with stdin.
		{
			Name:  "binary_flag_stdin",
			Args:  []string{"-b"},
			Stdin: []byte("abc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: -t flag is default text mode, "HASH  FILENAME".
		{
			Name:    "text_flag_explicit",
			Args:    []string{"-t", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R3.2: --text flag is default text mode.
		{
			Name:    "text_flag_long",
			Args:    []string{"--text", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R3.2: -t overrides earlier -b.
		{
			Name:    "text_overrides_binary",
			Args:    []string{"-b", "-t", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R3.1: -b overrides earlier -t.
		{
			Name:    "binary_overrides_text",
			Args:    []string{"-t", "-b", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R3.3: --tag with -b produces BSD format (mode has no effect on tag).
		{
			Name:    "tag_with_binary",
			Args:    []string{"--tag", "-b", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R3.3: -b with --tag (reversed order).
		{
			Name:    "binary_with_tag",
			Args:    []string{"-b", "--tag", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R3.1: -b with multiple files.
		{
			Name:    "binary_multiple_files",
			Args:    []string{"-b", "a.txt", "b.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirMulti,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// setupCheckDir creates a directory with a file and valid checksum file.
func setupCheckDir(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "input.txt", content)
	hash := md5hex(content)
	checksumLine := fmt.Sprintf("%s  input.txt\n", hash)
	writeTestFile(t, dir, "checksums.txt", checksumLine)
	return dir
}

// setupCheckFailDir creates a directory with a mismatched checksum.
func setupCheckFailDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "input.txt", "hello world\n")
	// Use wrong hash to trigger FAILED
	checksumLine := "00000000000000000000000000000000  input.txt\n"
	writeTestFile(t, dir, "checksums.txt", checksumLine)
	return dir
}

// setupCheckWarnDir creates a directory with valid and malformed lines.
func setupCheckWarnDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "input.txt", "hello world\n")
	hash := md5hex("hello world\n")
	content := fmt.Sprintf("this is not a valid line\n%s  input.txt\n", hash)
	writeTestFile(t, dir, "checksums.txt", content)
	return dir
}

// setupCheckBSDDir creates a directory with a BSD tag format checksum file.
func setupCheckBSDDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "input.txt", "hello world\n")
	hash := md5hex("hello world\n")
	content := fmt.Sprintf("MD5 (input.txt) = %s\n", hash)
	writeTestFile(t, dir, "checksums.txt", content)
	return dir
}

// writeTestFile writes a file in the given directory.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
}
