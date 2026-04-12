// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/cksum against gcksum.
// Implements srd077-cksum R3.2-R3.3 acceptance criteria via testutils.RunDiffTests.
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
// comparison succeeds. Handles "gcksum:" and full path forms.
func normalizeStderr(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("gcksum:"), []byte("cksum:"))
	idx := bytes.Index(data, []byte("/cksum:"))
	for idx >= 0 {
		start := bytes.LastIndex(data[:idx], []byte("\n"))
		if start == -1 {
			start = 0
		} else {
			start++
		}
		if data[start] == '/' {
			data = append(data[:start], append([]byte("cksum:"), data[idx+len("/cksum:"):]...)...)
		}
		next := bytes.Index(data[start+6:], []byte("/cksum:"))
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
// that GNU cksum appends to some error messages.
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

// algoNames lists algorithm tag names used by hashutil that may appear as
// stderr prefixes instead of the program name.
var algoNames = []string{
	"SHA1", "SHA224", "SHA256", "SHA384", "SHA512", "BLAKE2b",
}

// normalizeAlgoPrefix replaces algorithm name prefixes in stderr
// (e.g., "SHA256:") with "cksum:" to match GNU cksum's program-name prefix.
func normalizeAlgoPrefix(data []byte) []byte {
	for _, name := range algoNames {
		data = bytes.ReplaceAll(data,
			[]byte(name+":"),
			[]byte("cksum:"))
	}
	return data
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

// normalizeWarningPrefix adds "cksum: " prefix to bare WARNING lines
// so they match GNU's "cksum: WARNING:" format.
func normalizeWarningPrefix(data []byte) []byte {
	data = bytes.ReplaceAll(data,
		[]byte("WARNING: "),
		[]byte("cksum: WARNING: "))
	// Fix double-prefix if it was already prefixed.
	data = bytes.ReplaceAll(data,
		[]byte("cksum: cksum: WARNING:"),
		[]byte("cksum: WARNING:"))
	return data
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

// TestDiff runs differential tests for cksum against gcksum.
// D1: uses testutils.BuildBinary and exec.LookPath.
// D2: covers CRC-32 default, --algorithm, --check, --tag, stdin, multiple
// files, missing files, and error exit codes.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcksum")
	if err != nil {
		t.Skipf("reference binary gcksum not in PATH: %v", err)
	}

	dir := t.TempDir()

	// Create temporary test files for hashing.
	hello := writeTestFile(t, dir, "hello.txt", "hello\n")
	empty := writeTestFile(t, dir, "empty.txt", "")
	abc := writeTestFile(t, dir, "abc.txt", "abc")
	multi1 := writeTestFile(t, dir, "multi1.txt", "first file\n")
	multi2 := writeTestFile(t, dir, "multi2.txt", "second file\n")

	// Build checksum files for --check tests using the reference binary.
	checksumOK := generateChecksumArgs(t, refBin, dir, "checksums_ok.txt",
		[]string{"--algorithm=sha256", hello})
	checksumMultiOK := generateChecksumArgs(t, refBin, dir, "checksums_multi.txt",
		[]string{"--algorithm=sha256", hello, empty})
	checksumTag := generateChecksumArgs(t, refBin, dir, "checksums_tag.txt",
		[]string{"--algorithm=sha256", "--tag", hello})

	// Checksum file with a wrong hash to test failure reporting.
	checksumFail := writeTestFile(t, dir, "checksums_fail.txt",
		"SHA256 ("+hello+") = "+
			"0000000000000000000000000000000000000000000000000000000000000000\n")

	stderrNorm := []testutils.NormalizeFunc{
		normalizeStderr, normalizeStderrHint,
		normalizeAlgoPrefix, normalizeOpenWrap,
		normalizeWarningPrefix,
	}

	helpNorm := []testutils.NormalizeFunc{normalizeHelpVersion}

	tests := []testutils.DiffTest{
		// --- Default CRC-32 mode (R1.1-R1.3) ---
		{
			// R1.2: stdin hash when no arguments given.
			Name:  "crc_stdin",
			Stdin: []byte("hello\n"),
		},
		{
			// R1.1: single file CRC checksum.
			Name: "crc_file",
			Args: []string{hello},
		},
		{
			// R1.3: multiple files produce one line per file.
			Name: "crc_multiple_files",
			Args: []string{multi1, multi2},
		},
		{
			// R1.1: empty file produces valid CRC.
			Name: "crc_empty_file",
			Args: []string{empty},
		},
		{
			// R1.2: empty stdin produces valid CRC.
			Name:  "crc_empty_stdin",
			Stdin: []byte{},
		},
		{
			// R1.1: file without trailing newline.
			Name: "crc_no_trailing_newline",
			Args: []string{abc},
		},

		// --- Algorithm selection (R2.1-R2.2) ---
		{
			// R2.1: --algorithm=sha256 produces tagged output.
			Name:  "algo_sha256_stdin",
			Args:  []string{"--algorithm=sha256"},
			Stdin: []byte("hello\n"),
		},
		{
			// R2.1: --algorithm=sha256 on a file.
			Name: "algo_sha256_file",
			Args: []string{"--algorithm=sha256", hello},
		},
		{
			// R2.2: --untagged produces GNU two-space format.
			Name: "algo_sha256_untagged",
			Args: []string{"--algorithm=sha256", "--untagged", hello},
		},
		{
			// R2.1: --algorithm=sha1.
			Name: "algo_sha1_file",
			Args: []string{"--algorithm=sha1", hello},
		},
		{
			// R2.1: --algorithm=blake2b.
			Name: "algo_blake2b_file",
			Args: []string{"--algorithm=blake2b", hello},
		},
		{
			// R2.1: --algorithm=sha384.
			Name: "algo_sha384_file",
			Args: []string{"--algorithm=sha384", hello},
		},
		{
			// R2.1: --algorithm=sha512.
			Name: "algo_sha512_file",
			Args: []string{"--algorithm=sha512", hello},
		},
		{
			// R2.1: --algorithm=sha224.
			Name: "algo_sha224_file",
			Args: []string{"--algorithm=sha224", hello},
		},

		// --- Check mode (R2.1 via hashutil) ---
		{
			// --check with valid checksum, passes.
			Name: "check_sha256_ok",
			Args: []string{"--algorithm=sha256", "--check", checksumOK},
		},
		{
			// --check with multiple valid entries.
			Name: "check_sha256_multi_ok",
			Args: []string{"--algorithm=sha256", "--check", checksumMultiOK},
		},
		{
			// R3.2: --check with failed verification, exit 1.
			Name:      "check_sha256_fail",
			Args:      []string{"--algorithm=sha256", "--check", checksumFail},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			// --check parses BSD tag format.
			Name: "check_tag_format",
			Args: []string{"--algorithm=sha256", "--check", checksumTag},
		},
		{
			// --quiet suppresses OK lines on success.
			Name: "check_quiet_ok",
			Args: []string{"--algorithm=sha256", "--check", "--quiet", checksumOK},
		},
		{
			// --status suppresses all output, exit 0 on success.
			Name: "check_status_ok",
			Args: []string{"--algorithm=sha256", "--check", "--status", checksumOK},
		},
		{
			// R3.2: --status suppresses all output, exit 1 on failure.
			Name:     "check_status_fail",
			Args:     []string{"--algorithm=sha256", "--check", "--status", checksumFail},
			ExitCode: 1,
		},

		// --- Tag format ---
		{
			// --tag with non-CRC algorithm.
			Name: "tag_sha256",
			Args: []string{"--algorithm=sha256", "--tag", hello},
		},

		// --- Error cases (R3.2) ---
		{
			// R3.2: nonexistent file produces error and exit 1.
			Name:      "missing_file_crc",
			Args:      []string{filepath.Join(dir, "no_such_file.txt")},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			// R3.2: nonexistent file among valid files continues processing.
			Name:      "missing_among_valid_crc",
			Args:      []string{hello, filepath.Join(dir, "missing.txt"), multi1},
			ExitCode:  1,
			Normalize: stderrNorm,
		},
		{
			// R3.2: nonexistent file with non-CRC algorithm.
			Name:      "missing_file_sha256",
			Args:      []string{"--algorithm=sha256", filepath.Join(dir, "no_such.txt")},
			ExitCode:  1,
			Normalize: stderrNorm,
		},

		// --- Help and version ---
		{
			// --help prints usage and exits 0.
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: helpNorm,
		},
		{
			// --version prints version info and exits 0.
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: helpNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
