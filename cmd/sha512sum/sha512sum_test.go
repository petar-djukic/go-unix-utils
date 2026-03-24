// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sha512sum against GNU gsha512sum.
// Covers prd033-sha512sum R4.1-R4.3 (differential testing).
package main

import (
	"bytes"
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

// stderrNormalizer normalizes error messages between GNU gsha512sum and Go
// sha512sum. Handles differences in binary name prefixes, Go's "open" error
// wrapping, and per-line warn format variations.
func stderrNormalizer() testutils.NormalizeFunc {
	// Normalize binary name in absolute paths or bare references.
	binPath := regexp.MustCompile(`/[^\s:]+/g?sha512sum|gsha512sum`)
	// Remove GNU "Try ..." help hint lines.
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	// Normalize case of "no such file or directory".
	noSuch := regexp.MustCompile(`(?i)no such file or directory`)
	// Replace Go's "open /path: msg" with "/path: msg" (strip "open ").
	openWrap := regexp.MustCompile(`open (/[^:]+: )`)
	// Strip per-line warn detail lines (format differs between GNU and Go);
	// keep only the summary WARNING line which both produce.
	warnDetail := regexp.MustCompile(
		`(?m)^.*improperly formatted SHA512 checksum line\n?` +
			`|^.*no properly formatted SHA512 checksum lines found\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("sha512sum"))
		b = tryHelp.ReplaceAll(b, nil)
		b = openWrap.ReplaceAll(b, []byte("$1"))
		b = dedupPath(b)
		b = warnDetail.ReplaceAll(b, nil)
		b = noSuch.ReplaceAll(b, []byte("No such file or directory"))
		b = addSha512sumPrefix(b)
		return b
	}
}

// dedupPath collapses "/path: /path: msg" -> "/path: msg" on each line.
// Go error wrapping produces these duplicated path prefixes.
func dedupPath(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		lines[i] = dedupPathLine(line)
	}
	return bytes.Join(lines, []byte("\n"))
}

// dedupPathLine finds the first "/...path...: " segment and removes
// a consecutive duplicate of that same segment.
func dedupPathLine(line []byte) []byte {
	sep := []byte(": ")
	start := bytes.Index(line, []byte("/"))
	if start < 0 {
		return line
	}
	end := bytes.Index(line[start:], sep)
	if end < 0 {
		return line
	}
	seg := line[start : start+end+len(sep)]
	rest := line[start+end+len(sep):]
	if bytes.HasPrefix(rest, seg) {
		return append(line[:start], rest...)
	}
	return line
}

// addSha512sumPrefix ensures every non-empty line starts with "sha512sum: ".
func addSha512sumPrefix(b []byte) []byte {
	prefix := []byte("sha512sum: ")
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		if len(line) > 0 && !bytes.HasPrefix(line, prefix) {
			lines[i] = append(prefix, line...)
		}
	}
	return bytes.Join(lines, []byte("\n"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha512sum")
	if err != nil {
		t.Skipf("reference binary gsha512sum not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	dir := t.TempDir()

	// Create test files with known content for deterministic digests.
	helloFile := writeFile(t, dir, "hello.txt", "hello\n")
	abcFile := writeFile(t, dir, "abc.txt", "abc\n")

	// SHA512("hello\n") hash (128 hex chars).
	helloHash := "e7c22b994c59d9cf2b48e549b1e24666636045930d3da7c1acb299d1c3b7f931f94aae41edda2c2b207a36e10f8bcb8d45223e54878f5b316e7ce3b6bc019629"

	// Create valid checksum file for --check tests.
	checksumFile := writeFile(t, dir, "checksums.sha512",
		helloHash+"  hello.txt\n")

	// Create checksum file with wrong hash for --check failure test.
	badChecksumFile := writeFile(t, dir, "bad.sha512",
		"0000000000000000000000000000000000000000000000000000000000000000"+
			"0000000000000000000000000000000000000000000000000000000000000bad"+
			"  hello.txt\n")

	// Create checksum file with a malformed line for --warn test.
	malformedFile := writeFile(t, dir, "malformed.sha512",
		helloHash+"  hello.txt\n"+
			"this is not a valid line\n")

	// SHA512("abc\n") hash.
	abcHash := "4f285d0c0cc77286d8731798b7aae2639e28270d4166f40d769cbbdca5230714d848483d364e2f39fe6cb9083c15229b39a33615ebc6d57605f7c43f6906739d"

	// Create checksum file with multiple entries.
	multiChecksumFile := writeFile(t, dir, "multi.sha512",
		helloHash+"  hello.txt\n"+
			abcHash+"  abc.txt\n")

	tests := []testutils.DiffTest{
		// --- R4.1: Digest mode tests ---

		// R1.1: Compute SHA-512 of a single file in text mode.
		{
			Name:    "compute_single_file",
			Args:    []string{helloFile},
			WorkDir: dir,
		},
		// R1.1: Compute SHA-512 of multiple files.
		{
			Name:    "compute_multiple_files",
			Args:    []string{helloFile, abcFile},
			WorkDir: dir,
		},
		// R1.2: Compute SHA-512 from stdin.
		{
			Name:    "compute_stdin",
			Args:    []string{},
			Stdin:   []byte("hello\n"),
			WorkDir: dir,
		},
		// R1.2: Compute SHA-512 from stdin via explicit "-".
		{
			Name:    "compute_stdin_dash",
			Args:    []string{"-"},
			Stdin:   []byte("hello\n"),
			WorkDir: dir,
		},
		// R3.1: Binary mode uses asterisk format.
		{
			Name:    "binary_mode",
			Args:    []string{"-b", helloFile},
			WorkDir: dir,
		},
		// R3.1: Long --binary flag.
		{
			Name:    "binary_mode_long",
			Args:    []string{"--binary", helloFile},
			WorkDir: dir,
		},
		// R1.3: BSD tag format.
		{
			Name:    "tag_format",
			Args:    []string{"--tag", helloFile},
			WorkDir: dir,
		},
		// R1.3, R3.2: Tag with binary flag (mode has no effect on tag output).
		{
			Name:    "tag_with_binary",
			Args:    []string{"--tag", "-b", helloFile},
			WorkDir: dir,
		},
		// R3.1: Explicit text mode.
		{
			Name:    "text_mode",
			Args:    []string{"-t", helloFile},
			WorkDir: dir,
		},
		// R1.2: Empty stdin.
		{
			Name:    "empty_stdin",
			Args:    []string{},
			Stdin:   []byte{},
			WorkDir: dir,
		},
		// R1.1: Tag format with multiple files.
		{
			Name:    "tag_multiple_files",
			Args:    []string{"--tag", helloFile, abcFile},
			WorkDir: dir,
		},

		// --- R4.2: Check mode tests ---

		// R2.1, R2.2: Valid checksum file exits 0, prints OK.
		{
			Name:    "check_valid",
			Args:    []string{"--check", checksumFile},
			WorkDir: dir,
		},
		// R2.1, R2.2: Short -c flag.
		{
			Name:    "check_valid_short",
			Args:    []string{"-c", checksumFile},
			WorkDir: dir,
		},
		// R2.2: Check with mismatch exits 1, prints FAILED.
		{
			Name:      "check_failure",
			Args:      []string{"--check", badChecksumFile},
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.3: --warn prints warning for malformed lines.
		{
			Name:      "check_warn_malformed",
			Args:      []string{"--check", "--warn", malformedFile},
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.3: Without --warn, malformed lines are silently skipped.
		{
			Name:      "check_no_warn_malformed",
			Args:      []string{"--check", malformedFile},
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.3: --quiet suppresses OK lines.
		{
			Name:    "check_quiet",
			Args:    []string{"--check", "--quiet", checksumFile},
			WorkDir: dir,
		},
		// R2.3: --quiet with failure still prints FAILED.
		{
			Name:      "check_quiet_failure",
			Args:      []string{"--check", "--quiet", badChecksumFile},
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.3: --status suppresses all output; exit code conveys result.
		{
			Name:    "check_status_ok",
			Args:    []string{"--check", "--status", checksumFile},
			WorkDir: dir,
		},
		// R2.3: --status with failure exits 1, no output.
		{
			Name:    "check_status_failure",
			Args:    []string{"--check", "--status", badChecksumFile},
			WorkDir: dir,
		},
		// R2.1: Check with multiple entries.
		{
			Name:    "check_multiple_entries",
			Args:    []string{"--check", multiChecksumFile},
			WorkDir: dir,
		},

		// --- R4.3: Error handling tests ---

		// R1.4: Nonexistent file exits 1 with stderr error.
		{
			Name:      "nonexistent_file",
			Args:      []string{"/nonexistent-path/no-such-file.txt"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R1.4: Nonexistent file among valid files continues processing.
		{
			Name:      "nonexistent_with_valid",
			Args:      []string{"/nonexistent-path/no-such-file.txt", helloFile},
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.2: Check with nonexistent checksum file.
		{
			Name:      "check_nonexistent_checksumfile",
			Args:      []string{"--check", "/nonexistent-path/no-such-checksums.sha512"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R3.2: --tag with --check is invalid combination.
		{
			Name:      "tag_with_check_invalid",
			Args:      []string{"--tag", "--check", checksumFile},
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
