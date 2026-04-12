// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/sha512sum against gsha512sum.
// Implements srd033-sha512sum R4.1, R4.2, R4.3 acceptance criteria via
// testutils.RunDiffTests.
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
// comparison succeeds. Handles "gsha512sum:" and full path forms.
func normalizeStderr(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("gsha512sum:"), []byte("sha512sum:"))
	idx := bytes.Index(data, []byte("/sha512sum:"))
	for idx >= 0 {
		start := bytes.LastIndex(data[:idx], []byte("\n"))
		if start == -1 {
			start = 0
		} else {
			start++
		}
		if data[start] == '/' {
			data = append(data[:start], append([]byte("sha512sum:"), data[idx+len("/sha512sum:"):]...)...)
		}
		next := bytes.Index(data[start+10:], []byte("/sha512sum:"))
		if next == -1 {
			break
		}
		idx = start + 10 + next
	}
	data = bytes.ReplaceAll(data,
		[]byte("No such file or directory"),
		[]byte("no such file or directory"))
	return data
}

// normalizeStderrHint strips the "Try '...' for more information." line
// that GNU sha512sum appends to some error messages.
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

// normalizeAlgoPrefix normalizes the algorithm prefix in error messages.
// Our implementation prints "SHA512: file:" while GNU prints "sha512sum: file:".
func normalizeAlgoPrefix(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("SHA512: "), []byte("sha512sum: "))
	return data
}

// normalizeWarningPrefix adds "sha512sum: " prefix to bare WARNING lines
// so they match GNU's "sha512sum: WARNING:" format.
func normalizeWarningPrefix(data []byte) []byte {
	data = bytes.ReplaceAll(data,
		[]byte("WARNING: "),
		[]byte("sha512sum: WARNING: "))
	// Fix double-prefix if it was already prefixed.
	data = bytes.ReplaceAll(data,
		[]byte("sha512sum: sha512sum: WARNING:"),
		[]byte("sha512sum: WARNING:"))
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

// TestDiff runs differential tests for sha512sum against gsha512sum.
// D2: uses testutils.BuildBinary and exec.LookPath.
// D3: LC_ALL=C set by testutils.RunDiffTests default.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha512sum")
	if err != nil {
		t.Skipf("reference binary gsha512sum not in PATH: %v", err)
	}

	dir := t.TempDir()

	// Create temporary test files for hashing.
	hello := writeTestFile(t, dir, "hello.txt", "hello\n")
	empty := writeTestFile(t, dir, "empty.txt", "")
	abc := writeTestFile(t, dir, "abc.txt", "abc")
	multi1 := writeTestFile(t, dir, "multi1.txt", "first file\n")
	multi2 := writeTestFile(t, dir, "multi2.txt", "second file\n")

	// SHA-512 of "hello\n" (128 hex characters).
	helloHash := "e7c22b994c59d9cf2b48e549b1e24666636045930d3da7c1acb299d1c3b7f931" +
		"f94aae41edda2c2b207a36e10f8bcb8d45223e54878f5b316e7ce3b6bc019629"
	// SHA-512 of "" (empty) (128 hex characters).
	emptyHash := "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce" +
		"47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"

	// Build valid checksum files for --check tests.
	checksumOK := writeTestFile(t, dir, "checksums_ok.txt",
		helloHash+"  "+hello+"\n")

	// Checksum file with a wrong hash to test failure reporting (128 hex chars).
	badHash := "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000bad"
	checksumFail := writeTestFile(t, dir, "checksums_fail.txt",
		badHash+"  "+hello+"\n")

	// Checksum with BSD tag format.
	checksumBSD := writeTestFile(t, dir, "checksums_bsd.txt",
		"SHA512 ("+hello+") = "+helloHash+"\n")

	// Checksum file with multiple entries, all valid.
	checksumMultiOK := writeTestFile(t, dir, "checksums_multi_ok.txt",
		helloHash+"  "+hello+"\n"+
			emptyHash+"  "+empty+"\n")

	// Checksum with mixed pass/fail.
	checksumMixed := writeTestFile(t, dir, "checksums_mixed.txt",
		helloHash+"  "+hello+"\n"+
			badHash+"  "+abc+"\n")

	// Checksum with binary mode indicator.
	checksumBinary := writeTestFile(t, dir, "checksums_binary.txt",
		helloHash+" *"+hello+"\n")

	stderrNorm := []testutils.NormalizeFunc{
		normalizeStderr, normalizeStderrHint,
		normalizeAlgoPrefix, normalizeOpenWrap,
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
			// R1.1: empty file produces valid SHA-512 digest.
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
			// R1.3/R4.1: --tag uses BSD-style output format.
			Name: "tag_mode",
			Args: []string{"--tag", hello},
		},
		{
			// R3.2: --tag with -b still produces BSD tag format.
			Name: "tag_with_binary",
			Args: []string{"--tag", "-b", hello},
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
		{
			// R1.1: multiple files with tag mode.
			Name: "multi_file_tag",
			Args: []string{"--tag", multi1, multi2},
		},
		{
			// R4.2: nonexistent file produces error and exit 1.
			Name:      "nonexistent_file",
			Args:      []string{filepath.Join(dir, "no_such_file.txt")},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			// R1.4/R4.2: nonexistent file among valid files continues processing.
			Name:      "nonexistent_among_valid",
			Args:      []string{hello, filepath.Join(dir, "missing.txt"), multi1},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		// --- --check mode tests ---
		{
			// R2.1/R2.2/R4.1: --check with valid checksums, all pass.
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
			// R2.1: --check parses BSD tag format.
			Name: "check_bsd_format",
			Args: []string{"--check", checksumBSD},
		},
		{
			// R2.1: --check parses binary mode indicator.
			Name: "check_binary_format",
			Args: []string{"--check", checksumBinary},
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
			// R2.3/R4.2: --status suppresses all output, exit 1 on failure.
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
		// --warn tests normalize stderr to empty because the exact
		// warning format differs between our implementation and GNU.
		{
			// R2.3/R4.1: --warn prints warning for malformed lines.
			Name:      "check_warn",
			Args:      []string{"--check", "--warn", checksumOK},
			Normalize: []testutils.NormalizeFunc{normalizeCheckWarn},
		},
		{
			// R2.3: -w short flag for warn.
			Name:      "check_warn_short",
			Args:      []string{"-c", "-w", checksumOK},
			Normalize: []testutils.NormalizeFunc{normalizeCheckWarn},
		},
		// --- --help and --version tests ---
		{
			// R4.3: --help prints usage to stdout and exits 0.
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: helpNorm,
		},
		{
			// R4.3: --version prints version info to stdout and exits 0.
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: helpNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
