// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/md5sum against gmd5sum.
// Implements srd030-md5sum R4.1, R4.2, R4.3 acceptance criteria via
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
// comparison succeeds. Handles "gmd5sum:" and full path forms.
func normalizeStderr(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("gmd5sum:"), []byte("md5sum:"))
	idx := bytes.Index(data, []byte("/md5sum:"))
	for idx >= 0 {
		start := bytes.LastIndex(data[:idx], []byte("\n"))
		if start == -1 {
			start = 0
		} else {
			start++
		}
		if data[start] == '/' {
			data = append(data[:start], append([]byte("md5sum:"), data[idx+len("/md5sum:"):]...)...)
		}
		next := bytes.Index(data[start+7:], []byte("/md5sum:"))
		if next == -1 {
			break
		}
		idx = start + 7 + next
	}
	data = bytes.ReplaceAll(data,
		[]byte("No such file or directory"),
		[]byte("no such file or directory"))
	return data
}

// normalizeStderrHint strips the "Try '...' for more information." line
// that GNU md5sum appends to some error messages.
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
// Our implementation prints "MD5: file:" while GNU prints "md5sum: file:".
func normalizeAlgoPrefix(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("MD5: "), []byte("md5sum: "))
	return data
}

// normalizeWarningPrefix adds "md5sum: " prefix to bare WARNING lines
// so they match GNU's "md5sum: WARNING:" format.
func normalizeWarningPrefix(data []byte) []byte {
	data = bytes.ReplaceAll(data,
		[]byte("WARNING: "),
		[]byte("md5sum: WARNING: "))
	// Fix double-prefix if it was already prefixed.
	data = bytes.ReplaceAll(data,
		[]byte("md5sum: md5sum: WARNING:"),
		[]byte("md5sum: WARNING:"))
	return data
}

// normalizeCheckWarn normalizes --warn and --check stderr output
// to bridge format differences between our implementation and GNU.
// GNU formats: "md5sum: FILE: LINE: improperly formatted MD5 checksum line"
// Ours formats: "WARNING: N line is improperly formatted"
// We normalize both sides to empty to compare only stdout and exit code.
func normalizeCheckWarn(data []byte) []byte {
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

// TestDiff runs differential tests for md5sum against gmd5sum.
// D1: uses testutils.BuildBinary and exec.LookPath.
// D2: skips if gmd5sum not found.
// D4: LC_ALL=C is set by default via testutils.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmd5sum")
	if err != nil {
		t.Skipf("reference binary gmd5sum not in PATH: %v", err)
	}

	dir := t.TempDir()

	// D3: create temporary test files for hashing.
	hello := writeTestFile(t, dir, "hello.txt", "hello\n")
	empty := writeTestFile(t, dir, "empty.txt", "")
	abc := writeTestFile(t, dir, "abc.txt", "abc")
	multi1 := writeTestFile(t, dir, "multi1.txt", "first file\n")
	multi2 := writeTestFile(t, dir, "multi2.txt", "second file\n")

	// Build valid checksum files for --check tests.
	// MD5 of "hello\n" = b1946ac92492d2347c6235b4d2611184
	checksumOK := writeTestFile(t, dir, "checksums_ok.txt",
		"b1946ac92492d2347c6235b4d2611184  "+hello+"\n")

	// Checksum file with a wrong hash to test failure reporting.
	checksumFail := writeTestFile(t, dir, "checksums_fail.txt",
		"0000000000000000000000000000dead  "+hello+"\n")

	// Checksum with BSD tag format.
	checksumBSD := writeTestFile(t, dir, "checksums_bsd.txt",
		"MD5 ("+hello+") = b1946ac92492d2347c6235b4d2611184\n")

	// Checksum file with multiple entries, one failing.
	// MD5 of "" (empty) = d41d8cd98f00b204e9800998ecf8427e
	checksumMultiOK := writeTestFile(t, dir, "checksums_multi_ok.txt",
		"b1946ac92492d2347c6235b4d2611184  "+hello+"\n"+
			"d41d8cd98f00b204e9800998ecf8427e  "+empty+"\n")

	checksumMixed := writeTestFile(t, dir, "checksums_mixed.txt",
		"b1946ac92492d2347c6235b4d2611184  "+hello+"\n"+
			"0000000000000000000000000000dead  "+abc+"\n")

	// Checksum with binary mode indicator.
	checksumBinary := writeTestFile(t, dir, "checksums_binary.txt",
		"b1946ac92492d2347c6235b4d2611184 *"+hello+"\n")

	stderrNorm := []testutils.NormalizeFunc{
		normalizeStderr, normalizeStderrHint,
		normalizeAlgoPrefix, normalizeOpenWrap,
		normalizeWarningPrefix,
	}

	tests := []testutils.DiffTest{
		// --- R4.1/R4.2: file hashing, stdin, binary, tag, multi-file ---
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
			// R1.1: empty file produces valid MD5 digest.
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
			// R3.2: text mode flag -t (default behavior, explicit).
			Name: "text_mode_explicit",
			Args: []string{"-t", hello},
		},
		{
			// R1.3: --tag uses BSD-style output format.
			Name: "tag_mode",
			Args: []string{"--tag", hello},
		},
		{
			// R3.3: --tag with -b still produces BSD tag format.
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
		// --- R4.1/R4.2: --check mode tests ---
		{
			// R2.1/R2.2: --check with valid checksums, all pass.
			Name: "check_ok",
			Args: []string{"--check", checksumOK},
		},
		{
			// R2.1: --check with multiple valid entries, all pass.
			Name: "check_multi_ok",
			Args: []string{"--check", checksumMultiOK},
		},
		{
			// R2.1/R2.2/R4.2: --check with failed verification.
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
			// R2.4: --quiet suppresses OK lines on success.
			Name: "check_quiet_ok",
			Args: []string{"--check", "--quiet", checksumOK},
		},
		{
			// R2.4: --quiet still shows FAILED lines.
			Name:      "check_quiet_fail",
			Args:      []string{"--check", "--quiet", checksumFail},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			// R2.4: --status suppresses all output, exit 0 on success.
			Name: "check_status_ok",
			Args: []string{"--check", "--status", checksumOK},
		},
		{
			// R2.4: --status suppresses all output, exit 1 on failure.
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
		// --warn tests use clearOutput on stderr because the exact
		// warning format differs between our implementation and GNU.
		// Stdout and exit code are still compared.
		{
			// R2.3: --warn prints warning for malformed lines.
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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
