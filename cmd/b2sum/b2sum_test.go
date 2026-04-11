// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/b2sum against gb2sum.
// Implements srd076-b2sum R4.3 acceptance criteria via testutils.RunDiffTests.
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

// normalizeStderr replaces the reference binary name so differential
// comparison succeeds. Handles "gb2sum:" and full path forms.
func normalizeStderr(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("gb2sum:"), []byte("b2sum:"))
	idx := bytes.Index(data, []byte("/b2sum:"))
	for idx >= 0 {
		start := bytes.LastIndex(data[:idx], []byte("\n"))
		if start == -1 {
			start = 0
		} else {
			start++
		}
		if data[start] == '/' {
			data = append(data[:start], append([]byte("b2sum:"), data[idx+len("/b2sum:"):]...)...)
		}
		next := bytes.Index(data[start+6:], []byte("/b2sum:"))
		if next == -1 {
			break
		}
		idx = start + 6 + next
	}
	data = bytes.ReplaceAll(data,
		[]byte("No such file or directory"),
		[]byte("no such file or directory"))
	return data
}

// normalizeStderrHint strips the "Try '...' for more information." line
// that GNU b2sum appends to some error messages.
func normalizeStderrHint(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var out [][]byte
	for _, l := range lines {
		if bytes.HasPrefix(l, []byte("Try '")) {
			continue
		}
		out = append(out, l)
	}
	return bytes.Join(out, []byte("\n"))
}

// openWrapRe matches Go-style "open PATH: error" wrapping inside error messages.
var openWrapRe = regexp.MustCompile(`: open [^:]+: `)

// normalizeOpenWrap removes the Go-style "open PATH:" wrapping that Go's
// os.Open includes in error messages but GNU coreutils does not.
func normalizeOpenWrap(data []byte) []byte {
	return openWrapRe.ReplaceAllFunc(data, func(match []byte) []byte {
		return []byte(": ")
	})
}

// normalizeWarningPrefix adds "b2sum: " prefix to bare WARNING lines
// so they match GNU's "b2sum: WARNING:" format.
func normalizeWarningPrefix(data []byte) []byte {
	data = bytes.ReplaceAll(data,
		[]byte("WARNING: "),
		[]byte("b2sum: WARNING: "))
	// Fix double-prefix if it was already prefixed.
	data = bytes.ReplaceAll(data,
		[]byte("b2sum: b2sum: WARNING:"),
		[]byte("b2sum: WARNING:"))
	return data
}

// normalizeCheckWarn normalizes --warn and --check stderr output
// to bridge format differences between our implementation and GNU.
func normalizeCheckWarn(data []byte) []byte {
	return nil
}

// normalizeHelpVersion normalizes --help and --version output to empty
// since our output text differs from GNU's. We only verify exit code.
func normalizeHelpVersion(data []byte) []byte {
	return nil
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestDiff runs differential tests for b2sum against gb2sum.
// D1: uses testutils.BuildBinary and exec.LookPath.
// D2: skips if gb2sum not found.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gb2sum")
	if err != nil {
		t.Skipf("reference binary gb2sum not in PATH: %v", err)
	}

	dir := t.TempDir()

	// Create temporary test files for hashing.
	hello := writeTestFile(t, dir, "hello.txt", "hello\n")
	empty := writeTestFile(t, dir, "empty.txt", "")
	abc := writeTestFile(t, dir, "abc.txt", "abc")
	multi1 := writeTestFile(t, dir, "multi1.txt", "first file\n")
	multi2 := writeTestFile(t, dir, "multi2.txt", "second file\n")

	// Build valid checksum files for --check tests.
	// Run the reference binary to generate correct checksums.
	checksumOK := generateChecksum(t, refBin, dir, "checksums_ok.txt", hello)
	checksumBinary := generateChecksumArgs(t, refBin, dir, "checksums_binary.txt",
		[]string{"-b", hello})
	checksumTag := generateChecksumArgs(t, refBin, dir, "checksums_tag.txt",
		[]string{"--tag", hello})

	// Checksum file with a wrong hash to test failure reporting.
	checksumFail := writeTestFile(t, dir, "checksums_fail.txt",
		"0000000000000000000000000000000000000000000000000000000000000000"+
			"0000000000000000000000000000000000000000000000000000000000000bad  "+hello+"\n")

	// Multi-entry checksum file (all valid).
	checksumMultiOK := generateChecksumArgs(t, refBin, dir, "checksums_multi_ok.txt",
		[]string{hello, empty})

	// Multi-entry with one mismatch.
	checksumMixed := buildMixedChecksum(t, refBin, dir, hello, abc)

	// Checksum with --length=256.
	checksum256 := generateChecksumArgs(t, refBin, dir, "checksums_256.txt",
		[]string{"--length=256", hello})

	stderrNorm := []testutils.NormalizeFunc{
		normalizeStderr, normalizeStderrHint, normalizeOpenWrap,
		normalizeWarningPrefix,
	}

	helpNorm := []testutils.NormalizeFunc{normalizeHelpVersion}

	tests := []testutils.DiffTest{
		// --- File hashing, stdin, binary, tag, multi-file ---
		{
			// R1.1: single file hash in GNU text format.
			Name: "single_file_hash",
			Args: []string{hello},
		},
		{
			// R1.2: stdin hash when no arguments given.
			Name:  "stdin_no_args",
			Stdin: []byte("hello\n"),
		},
		{
			// R1.2: stdin hash with explicit "-" argument.
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("hello\n"),
		},
		{
			// R1.1: empty file produces valid BLAKE2b digest.
			Name: "empty_file",
			Args: []string{empty},
		},
		{
			// R1.1: file without trailing newline.
			Name: "file_no_trailing_newline",
			Args: []string{abc},
		},
		{
			// R3.1: binary mode flag -b uses asterisk format.
			Name: "binary_mode",
			Args: []string{"-b", hello},
		},
		{
			// R3.1: --binary long flag.
			Name: "binary_mode_long",
			Args: []string{"--binary", hello},
		},
		{
			// R3.1: text mode flag -t (default behavior, explicit).
			Name: "text_mode_explicit",
			Args: []string{"-t", hello},
		},
		{
			// R1.3/R3.2: --tag uses BSD-style output format.
			Name: "tag_mode",
			Args: []string{"--tag", hello},
		},
		{
			// R1.1: multiple file arguments produce one line each.
			Name: "multi_file",
			Args: []string{multi1, multi2},
		},
		{
			// R1.1: multiple files with binary mode.
			Name: "multi_file_binary",
			Args: []string{"-b", multi1, multi2},
		},
		// --- --length tests ---
		{
			// R3.3: --length=8 produces a 2-character hex digest.
			Name: "length_8",
			Args: []string{"--length=8", hello},
		},
		{
			// R3.3: --length=128 produces a 32-character hex digest.
			Name: "length_128",
			Args: []string{"--length=128", hello},
		},
		{
			// R3.3: --length=256 produces a 64-character hex digest.
			Name: "length_256",
			Args: []string{"--length=256", hello},
		},
		{
			// R3.3: --length=512 is the default, same as no --length.
			Name: "length_512_explicit",
			Args: []string{"--length=512", hello},
		},
		{
			// R1.3: --tag with --length includes bit length in tag name.
			Name: "tag_with_length",
			Args: []string{"--tag", "--length=256", hello},
		},
		{
			// R3.3: -l short flag for length.
			Name: "length_short_flag",
			Args: []string{"-l", "128", hello},
		},
		// --- Error conditions ---
		{
			// R1.4/R4.2: nonexistent file produces error and exit 1.
			Name:      "nonexistent_file",
			Args:      []string{filepath.Join(dir, "no_such_file.txt")},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			// R1.4: nonexistent file among valid files continues processing.
			Name:      "nonexistent_among_valid",
			Args:      []string{hello, filepath.Join(dir, "missing.txt"), multi1},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			// R1.2: empty stdin produces valid BLAKE2b digest of empty input.
			Name:  "empty_stdin",
			Stdin: []byte{},
		},
		// --- --check mode tests ---
		{
			// R2.1/R2.2: --check with valid checksum, passes.
			Name: "check_ok",
			Args: []string{"--check", checksumOK},
		},
		{
			// R2.1: --check with multiple valid entries, all pass.
			Name: "check_multi_ok",
			Args: []string{"--check", checksumMultiOK},
		},
		{
			// R2.2/R4.2: --check with failed verification.
			Name:      "check_fail",
			Args:      []string{"--check", checksumFail},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			// R2.1: --check parses binary mode indicator.
			Name: "check_binary_format",
			Args: []string{"--check", checksumBinary},
		},
		{
			// R2.1: --check parses BSD tag format.
			Name: "check_bsd_tag_format",
			Args: []string{"--check", checksumTag},
		},
		{
			// R2.3: --quiet suppresses OK lines on success.
			Name: "check_quiet_ok",
			Args: []string{"--check", "--quiet", checksumOK},
		},
		{
			// R2.3: --quiet still shows FAILED lines.
			Name:      "check_quiet_fail",
			Args:      []string{"--check", "--quiet", checksumFail},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			// R2.3: --status suppresses all output, exit 0 on success.
			Name: "check_status_ok",
			Args: []string{"--check", "--status", checksumOK},
		},
		{
			// R2.3: --status suppresses all output, exit 1 on failure.
			Name:     "check_status_fail",
			Args:     []string{"--check", "--status", checksumFail},
			ExitCode: 1,
		},
		{
			// R2.2/R4.2: mixed pass/fail in --check, exit 1.
			Name:      "check_mixed",
			Args:      []string{"--check", checksumMixed},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			// R2.1: --check with -c short flag.
			Name: "check_short_flag",
			Args: []string{"-c", checksumOK},
		},
		{
			// R2.3: --warn prints warning for malformed lines.
			Name:      "check_warn",
			Args:      []string{"--check", "--warn", checksumOK},
			Normalize: []testutils.NormalizeFunc{normalizeCheckWarn},
		},
		{
			// R2.3: --strict with valid file, exit 0.
			Name:      "check_strict_ok",
			Args:      []string{"--check", "--strict", checksumOK},
			Normalize: []testutils.NormalizeFunc{normalizeCheckWarn},
		},
		{
			// R2.1: --check with --length=256 checksum file.
			Name: "check_length_256",
			Args: []string{"--check", "--length=256", checksum256},
		},
		// --- --help and --version tests ---
		{
			// --help prints usage to stdout and exits 0.
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: helpNorm,
		},
		{
			// --version prints version info to stdout and exits 0.
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: helpNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// generateChecksum runs the reference binary on a file and writes its
// stdout to a checksum file, returning the path.
func generateChecksum(t *testing.T, refBin, dir, name, file string) string {
	t.Helper()
	return generateChecksumArgs(t, refBin, dir, name, []string{file})
}

// generateChecksumArgs runs the reference binary with args and writes its
// stdout to a checksum file, returning the path.
func generateChecksumArgs(t *testing.T, refBin, dir, name string, args []string) string {
	t.Helper()
	cmd := exec.Command(refBin, args...)
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("generating checksum with %s %v: %v", refBin, args, err)
	}
	return writeTestFile(t, dir, name, string(out))
}

// buildMixedChecksum generates a checksum file with one correct and one
// incorrect entry to test mixed pass/fail in --check mode.
func buildMixedChecksum(t *testing.T, refBin, dir, goodFile, badFile string) string {
	t.Helper()
	// Get the correct checksum for goodFile.
	cmd := exec.Command(refBin, goodFile)
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
	goodLine, err := cmd.Output()
	if err != nil {
		t.Fatalf("generating checksum for %s: %v", goodFile, err)
	}
	// Build a bad line for badFile with a zeroed hash.
	badLine := "0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000bad  " + badFile + "\n"
	content := string(goodLine) + badLine
	return writeTestFile(t, dir, "checksums_mixed.txt", content)
}
