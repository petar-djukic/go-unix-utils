// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/md5sum against the GNU reference binary (gmd5sum).
// Implements prd030-md5sum R1-R4 verification.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces "gmd5sum:" with "md5sum:" in stderr output so that
// error messages from the reference binary match the Go binary's program name.
var normalizeProgramName testutils.NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gmd5sum:"), []byte("md5sum:"))
}

// normalizeErrMsg normalizes OS error message capitalization differences.
// GNU coreutils prints "No such file or directory" while Go's os package
// returns "no such file or directory".
var normalizeErrMsg testutils.NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("No such file or directory"), []byte("no such file or directory"))
}

// writeFile creates a file in dir with the given content.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmd5sum")
	if err != nil {
		t.Skipf("reference binary gmd5sum not in PATH: %v", err)
	}

	// Setup: directory with hello.txt containing "hello\n"
	dirFile := t.TempDir()
	writeFile(t, dirFile, "hello.txt", "hello\n")

	// Setup: directory with multiple files
	dirMulti := t.TempDir()
	writeFile(t, dirMulti, "a.txt", "hello\n")
	writeFile(t, dirMulti, "b.txt", "world\n")

	// Setup: directory with valid checksum file
	dirCheckOK := t.TempDir()
	writeFile(t, dirCheckOK, "hello.txt", "hello\n")
	writeFile(t, dirCheckOK, "checksums.md5", "b1946ac92492d2347c6235b4d2611184  hello.txt\n")

	// Setup: directory with invalid checksum file
	dirCheckBad := t.TempDir()
	writeFile(t, dirCheckBad, "hello.txt", "hello\n")
	writeFile(t, dirCheckBad, "bad.md5", "00000000000000000000000000000000  hello.txt\n")

	// Setup: directory for --tag test
	dirTag := t.TempDir()
	writeFile(t, dirTag, "hello.txt", "hello\n")

	// Setup: directory for nonexistent file test
	dirMissing := t.TempDir()

	// Setup: directory for --quiet check test
	dirQuiet := t.TempDir()
	writeFile(t, dirQuiet, "hello.txt", "hello\n")
	writeFile(t, dirQuiet, "checksums.md5", "b1946ac92492d2347c6235b4d2611184  hello.txt\n")

	// Setup: directory for --status check test
	dirStatus := t.TempDir()
	writeFile(t, dirStatus, "hello.txt", "hello\n")
	writeFile(t, dirStatus, "checksums.md5", "b1946ac92492d2347c6235b4d2611184  hello.txt\n")

	// Setup: directory for binary mode test
	dirBinary := t.TempDir()
	writeFile(t, dirBinary, "hello.txt", "hello\n")

	// Setup: directory for check with --status on failure
	dirStatusFail := t.TempDir()
	writeFile(t, dirStatusFail, "hello.txt", "hello\n")
	writeFile(t, dirStatusFail, "bad.md5", "00000000000000000000000000000000  hello.txt\n")

	tests := []testutils.DiffTest{
		// R1.2: stdin input, no file args.
		{
			Name:  "stdin_input",
			Args:  []string{},
			Stdin: []byte("hello\n"),
		},
		// R1.1: single file.
		{
			Name:    "single_file",
			Args:    []string{"hello.txt"},
			WorkDir: dirFile,
		},
		// R1.1: multiple files.
		{
			Name:    "multiple_files",
			Args:    []string{"a.txt", "b.txt"},
			WorkDir: dirMulti,
		},
		// R3.1: binary mode flag.
		{
			Name:    "binary_mode",
			Args:    []string{"-b", "hello.txt"},
			WorkDir: dirBinary,
		},
		// R1.3: --tag BSD-style output.
		{
			Name:    "tag_format",
			Args:    []string{"--tag", "hello.txt"},
			WorkDir: dirTag,
		},
		// R2.1, R2.2: check mode with valid checksums.
		{
			Name:    "check_valid",
			Args:    []string{"--check", "checksums.md5"},
			WorkDir: dirCheckOK,
		},
		// R2.2: check mode with invalid checksums.
		{
			Name:      "check_failure",
			Args:      []string{"--check", "bad.md5"},
			WorkDir:   dirCheckBad,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R1.4: nonexistent file.
		{
			Name:      "nonexistent_file",
			Args:      []string{"nosuchfile.txt"},
			WorkDir:   dirMissing,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeErrMsg},
		},
		// R2.4: --quiet in check mode (suppress OK lines).
		{
			Name:    "check_quiet",
			Args:    []string{"--check", "--quiet", "checksums.md5"},
			WorkDir: dirQuiet,
		},
		// R2.4: --status in check mode (no output, exit code only).
		{
			Name:    "check_status_ok",
			Args:    []string{"--check", "--status", "checksums.md5"},
			WorkDir: dirStatus,
		},
		// R2.4: --status in check mode with failure.
		{
			Name:     "check_status_fail",
			Args:     []string{"--check", "--status", "bad.md5"},
			WorkDir:  dirStatusFail,
			ExitCode: 1,
		},
		// R1.2: explicit stdin via "-".
		{
			Name:  "explicit_stdin",
			Args:  []string{"-"},
			Stdin: []byte("hello\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
