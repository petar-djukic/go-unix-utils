// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sha512sum against the GNU reference binary (gsha512sum).
// Implements prd033-sha512sum R1-R4 verification.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces "gsha512sum:" with "sha512sum:" in stderr output so that
// error messages from the reference binary match the Go binary's program name.
var normalizeProgramName testutils.NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gsha512sum:"), []byte("sha512sum:"))
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

// sha512 of "hello\n" = e7c22b994c59d9cf2b48e549b1e24666636045930d3da7c1acb299d1c3b7f931f94aae41edda2c2b207a36e10f8bcb8d45223e54878f5b316e7ce3b6bc019629d
const helloSHA512 = "e7c22b994c59d9cf2b48e549b1e24666636045930d3da7c1acb299d1c3b7f931f94aae41edda2c2b207a36e10f8bcb8d45223e54878f5b316e7ce3b6bc019629"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha512sum")
	if err != nil {
		t.Skipf("reference binary gsha512sum not in PATH: %v", err)
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
	writeFile(t, dirCheckOK, "checksums.sha512", helloSHA512+"  hello.txt\n")

	// Setup: directory with invalid checksum file
	dirCheckBad := t.TempDir()
	writeFile(t, dirCheckBad, "hello.txt", "hello\n")
	writeFile(t, dirCheckBad, "bad.sha512", "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000  hello.txt\n")

	// Setup: directory for --tag test
	dirTag := t.TempDir()
	writeFile(t, dirTag, "hello.txt", "hello\n")

	// Setup: directory for nonexistent file test
	dirMissing := t.TempDir()

	// Setup: directory for --quiet check test
	dirQuiet := t.TempDir()
	writeFile(t, dirQuiet, "hello.txt", "hello\n")
	writeFile(t, dirQuiet, "checksums.sha512", helloSHA512+"  hello.txt\n")

	// Setup: directory for --status check test
	dirStatus := t.TempDir()
	writeFile(t, dirStatus, "hello.txt", "hello\n")
	writeFile(t, dirStatus, "checksums.sha512", helloSHA512+"  hello.txt\n")

	// Setup: directory for binary mode test
	dirBinary := t.TempDir()
	writeFile(t, dirBinary, "hello.txt", "hello\n")

	// Setup: directory for check with --status on failure
	dirStatusFail := t.TempDir()
	writeFile(t, dirStatusFail, "hello.txt", "hello\n")
	writeFile(t, dirStatusFail, "bad.sha512", "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000  hello.txt\n")

	// Setup: directory for --warn test with malformed lines
	dirWarn := t.TempDir()
	writeFile(t, dirWarn, "hello.txt", "hello\n")
	writeFile(t, dirWarn, "warn.sha512", "not a valid line\n"+helloSHA512+"  hello.txt\n")

	// Setup: directory for --strict test
	dirStrict := t.TempDir()
	writeFile(t, dirStrict, "hello.txt", "hello\n")
	writeFile(t, dirStrict, "strict.sha512", "not a valid line\n"+helloSHA512+"  hello.txt\n")

	// Setup: directory for --ignore-missing test
	dirIgnore := t.TempDir()
	writeFile(t, dirIgnore, "hello.txt", "hello\n")
	writeFile(t, dirIgnore, "ignore.sha512", helloSHA512+"  hello.txt\n"+helloSHA512+"  missing.txt\n")

	// Setup: directory for text mode test
	dirText := t.TempDir()
	writeFile(t, dirText, "hello.txt", "hello\n")

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
		// R3.2: text mode flag (default behavior, explicit flag).
		{
			Name:    "text_mode",
			Args:    []string{"-t", "hello.txt"},
			WorkDir: dirText,
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
			Args:    []string{"--check", "checksums.sha512"},
			WorkDir: dirCheckOK,
		},
		// R2.2: check mode with invalid checksums.
		{
			Name:      "check_failure",
			Args:      []string{"--check", "bad.sha512"},
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
		// R2.3: --quiet in check mode (suppress OK lines).
		{
			Name:    "check_quiet",
			Args:    []string{"--check", "--quiet", "checksums.sha512"},
			WorkDir: dirQuiet,
		},
		// R2.3: --status in check mode (no output, exit code only).
		{
			Name:    "check_status_ok",
			Args:    []string{"--check", "--status", "checksums.sha512"},
			WorkDir: dirStatus,
		},
		// R2.3: --status in check mode with failure.
		{
			Name:     "check_status_fail",
			Args:     []string{"--check", "--status", "bad.sha512"},
			WorkDir:  dirStatusFail,
			ExitCode: 1,
		},
		// R1.2: explicit stdin via "-".
		{
			Name:  "explicit_stdin",
			Args:  []string{"-"},
			Stdin: []byte("hello\n"),
		},
		// R2.3: --warn flag for improperly formatted lines.
		{
			Name:      "check_warn",
			Args:      []string{"--check", "--warn", "warn.sha512"},
			WorkDir:   dirWarn,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4: --strict flag exits non-zero on improperly formatted lines.
		{
			Name:      "check_strict",
			Args:      []string{"--check", "--strict", "strict.sha512"},
			WorkDir:   dirStrict,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4: --ignore-missing skips missing files in check mode.
		{
			Name:      "check_ignore_missing",
			Args:      []string{"--check", "--ignore-missing", "ignore.sha512"},
			WorkDir:   dirIgnore,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
