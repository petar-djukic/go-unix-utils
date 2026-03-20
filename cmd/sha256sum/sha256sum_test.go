// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd032-sha256sum R1.1, R1.2, R1.3, R1.4,
// R2.1, R2.2, R2.3, R3.1, R3.2, R4.1, R4.2, R4.3.
package main

import (
	"crypto/sha256"
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
		writeTestFile(t, dir, name, content)
	}
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

// sha256hex computes the SHA-256 hex digest of content.
func sha256hex(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

// errLinePattern matches error lines from both gsha256sum and sha256sum.
var errLinePattern = regexp.MustCompile(`(?:g?sha256sum): [^\n]*\n`)

// tryLinePattern matches GNU "Try ..." help hint lines in stderr.
var tryLinePattern = regexp.MustCompile(`Try '[^\n]*' for more information\.\n`)

// stderrNormalizer replaces stderr error lines with a fixed placeholder
// to account for binary name and message format differences.
func stderrNormalizer(data []byte) []byte {
	data = errLinePattern.ReplaceAll(data, []byte("sha256sum: <ERROR>\n"))
	return tryLinePattern.ReplaceAll(data, nil)
}

// warnLinePattern matches warning lines about improperly formatted lines.
var warnLinePattern = regexp.MustCompile(
	`(?:g?sha256sum): [^:]+: \d+: improperly formatted[^\n]*\n`,
)

// warnNormalizer normalizes warning lines about malformed checksum lines.
func warnNormalizer(data []byte) []byte {
	return warnLinePattern.ReplaceAll(data, []byte("sha256sum: WARN_LINE\n"))
}

// failedSummaryPattern matches the summary line printed after --check.
var failedSummaryPattern = regexp.MustCompile(
	`(?:g?sha256sum): WARNING: \d+ computed checksum[^\n]*\n`,
)

// failedSummaryNormalizer normalizes the summary warning line.
func failedSummaryNormalizer(data []byte) []byte {
	return failedSummaryPattern.ReplaceAll(data, []byte("sha256sum: WARNING_SUMMARY\n"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha256sum")
	if err != nil {
		t.Skipf("reference binary gsha256sum not in PATH: %v", err)
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

	// R2.3: --quiet and --status test directories.
	dirCheckQuiet := setupCheckDir(t, "hello world\n")
	dirCheckStatus := setupCheckDir(t, "hello world\n")
	dirCheckStatusFail := setupCheckFailDir(t)

	// R2.1: BSD tag format check file.
	dirCheckBSD := setupCheckBSDDir(t)

	// R4.1: Check mode with multiple files all matching.
	dirCheckMulti := setupCheckMultiDir(t)

	// R4.2: Check mode referencing a file that does not exist.
	dirCheckMissing := setupCheckMissingRefDir(t)

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
		// R1.1: --binary long flag produces "HASH *FILENAME" format.
		{
			Name:    "binary_flag_long",
			Args:    []string{"--binary", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.1: -b with stdin.
		{
			Name:  "binary_flag_stdin",
			Args:  []string{"-b"},
			Stdin: []byte("abc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: -t flag is default text mode, "HASH  FILENAME".
		{
			Name:    "text_flag_explicit",
			Args:    []string{"-t", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.1: -t overrides earlier -b.
		{
			Name:    "text_overrides_binary",
			Args:    []string{"-b", "-t", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.1: -b overrides earlier -t.
		{
			Name:    "binary_overrides_text",
			Args:    []string{"-t", "-b", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.3: --tag with -b produces BSD format (tag takes precedence).
		{
			Name:    "tag_with_binary",
			Args:    []string{"--tag", "-b", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.1: -b with multiple files.
		{
			Name:    "binary_multiple_files",
			Args:    []string{"-b", "a.txt", "b.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirMulti,
		},
		// R1.4: Exit 0 when all files processed successfully.
		{
			Name:    "exit_zero_single_file",
			Args:    []string{"input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.4: Exit 1 when a file cannot be opened.
		{
			Name:      "exit_one_missing_file",
			Args:      []string{"no_such_file.txt"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			WorkDir:   dirSingle,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
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
		// R2.1: --check with -c short flag.
		{
			Name:      "check_short_flag",
			Args:      []string{"-c", "checksums.txt"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   dirCheck,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R2.3: --check --warn with malformed lines.
		{
			Name:    "check_warn",
			Args:    []string{"--check", "--warn", "checksums.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirCheckWarn,
			Normalize: []testutils.NormalizeFunc{
				warnNormalizer, stderrNormalizer, failedSummaryNormalizer,
			},
		},
		// R2.3: --check --quiet suppresses OK lines.
		{
			Name:      "check_quiet",
			Args:      []string{"--check", "--quiet", "checksums.txt"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   dirCheckQuiet,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R2.3: --check --status suppresses all output.
		{
			Name:      "check_status_pass",
			Args:      []string{"--check", "--status", "checksums.txt"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   dirCheckStatus,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R2.3: --check --status with failure, exit 1, no output.
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
			Name:    "r3_binary_flag",
			Args:    []string{"-b", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R3.1: -t (default) produces "HASH  FILENAME" format.
		{
			Name:    "r3_text_flag",
			Args:    []string{"-t", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R3.2: --tag rejects explicit -t (text mode).
		{
			Name:      "tag_rejects_text",
			Args:      []string{"-t", "--tag", "input.txt"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			WorkDir:   dirSingle,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R3.2: --tag rejects explicit -t even with -b before it.
		{
			Name:      "tag_rejects_binary_text_combo",
			Args:      []string{"-b", "-t", "--tag", "input.txt"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			WorkDir:   dirSingle,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R3.2: --tag with multiple files.
		{
			Name:    "tag_multiple_files",
			Args:    []string{"--tag", "a.txt", "b.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirMulti,
		},
		// R4.1: Exit 0 when all files processed successfully (multiple).
		{
			Name:    "exit_zero_multiple_files",
			Args:    []string{"a.txt", "b.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirMulti,
		},
		// R4.1: Exit 0 when all verified digests match in check mode.
		{
			Name:      "check_exit_zero_all_match",
			Args:      []string{"--check", "checksums.txt"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   dirCheckMulti,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.2: Exit 1 when checksum file references a missing file.
		{
			Name:     "check_missing_referenced_file",
			Args:     []string{"--check", "checksums.txt"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
			WorkDir:  dirCheckMissing,
			Normalize: []testutils.NormalizeFunc{
				stderrNormalizer, failedSummaryNormalizer,
			},
		},
		// R4.2: Exit 1 when all named files are missing.
		{
			Name:      "exit_one_all_missing",
			Args:      []string{"no1.txt", "no2.txt"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			WorkDir:   dirSingle,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// setupCheckDir creates a directory with a file and valid GNU checksum file.
func setupCheckDir(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "input.txt", content)
	hash := sha256hex(content)
	checksumLine := fmt.Sprintf("%s  input.txt\n", hash)
	writeTestFile(t, dir, "checksums.txt", checksumLine)
	return dir
}

// setupCheckFailDir creates a directory with a mismatched checksum.
func setupCheckFailDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "input.txt", "hello world\n")
	// Use wrong hash to trigger FAILED (64 hex zeros for SHA-256)
	wrong := "0000000000000000000000000000000000000000000000000000000000000000"
	checksumLine := fmt.Sprintf("%s  input.txt\n", wrong)
	writeTestFile(t, dir, "checksums.txt", checksumLine)
	return dir
}

// setupCheckWarnDir creates a directory with valid and malformed lines.
func setupCheckWarnDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "input.txt", "hello world\n")
	hash := sha256hex("hello world\n")
	content := fmt.Sprintf("this is not a valid line\n%s  input.txt\n", hash)
	writeTestFile(t, dir, "checksums.txt", content)
	return dir
}

// setupCheckBSDDir creates a directory with a BSD tag format checksum file.
func setupCheckBSDDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "input.txt", "hello world\n")
	hash := sha256hex("hello world\n")
	content := fmt.Sprintf("SHA256 (input.txt) = %s\n", hash)
	writeTestFile(t, dir, "checksums.txt", content)
	return dir
}

// setupCheckMultiDir creates a directory with multiple files and a valid checksum file.
// R4.1: used to verify exit 0 when all digests match.
func setupCheckMultiDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, dir, "a.txt", "aaa\n")
	writeTestFile(t, dir, "b.txt", "bbb\n")
	hashA := sha256hex("aaa\n")
	hashB := sha256hex("bbb\n")
	content := fmt.Sprintf("%s  a.txt\n%s  b.txt\n", hashA, hashB)
	writeTestFile(t, dir, "checksums.txt", content)
	return dir
}

// setupCheckMissingRefDir creates a checksum file referencing a nonexistent file.
// R4.2: used to verify exit 1 when a referenced file cannot be opened.
func setupCheckMissingRefDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	hash := sha256hex("anything\n")
	content := fmt.Sprintf("%s  nonexistent.txt\n", hash)
	writeTestFile(t, dir, "checksums.txt", content)
	return dir
}
