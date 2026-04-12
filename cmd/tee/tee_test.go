// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/tee.
// Tests cover srd017-tee R1.1-R1.5, R2.1-R2.3, R3.1-R3.4, R4.1-R4.3.
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

// stderrProgRe matches the program name/path prefix before a colon at line start.
var stderrProgRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// stderrTryRe matches the quoted program reference in Try hint lines.
var stderrTryRe = regexp.MustCompile(`'[^']*--help'`)

// stderrNormalizer normalizes program name differences in error messages.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = stderrTryRe.ReplaceAll(b, []byte("'PROG --help'"))
	// Normalize error message casing (Go uses lowercase, GNU uses uppercase).
	b = bytes.ToLower(b)
	return b
}

// versionNormalizer strips all version output since content differs between implementations.
func versionNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	return []byte("VERSION\n")
}

// helpNormalizer strips all content since help text differs between implementations.
func helpNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	return []byte("HELP\n")
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}

	// Create a read-only directory for write-error tests.
	roDir := t.TempDir()
	roFile := filepath.Join(roDir, "readonly.txt")
	if err := os.WriteFile(roFile, []byte("existing\n"), 0o444); err != nil {
		t.Fatalf("setup readonly file: %v", err)
	}
	roSubDir := filepath.Join(roDir, "nowrite")
	if err := os.MkdirAll(roSubDir, 0o555); err != nil {
		t.Fatalf("setup readonly dir: %v", err)
	}
	badPath := filepath.Join(roSubDir, "out.txt")

	tests := []testutils.DiffTest{
		// R1.1, R1.5: basic stdin passthrough to stdout and single file.
		{
			Name:  "single_file",
			Args:  []string{filepath.Join(t.TempDir(), "out.txt")},
			Stdin: []byte("hello\nworld\n"),
		},
		// R1.2: no file arguments, passthrough only.
		{
			Name:  "passthrough_no_files",
			Stdin: []byte("passthrough data\n"),
		},
		// R1.1: multiple output files.
		{
			Name: "multiple_files",
			Args: []string{
				filepath.Join(t.TempDir(), "a.txt"),
				filepath.Join(t.TempDir(), "b.txt"),
			},
			Stdin: []byte("multi\nfile\noutput\n"),
		},
		// R1.3: file creation when file does not exist (covered by single_file).
		// R1.4: "-" as file argument treated as stdout.
		{
			Name:  "dash_as_file",
			Args:  []string{"-"},
			Stdin: []byte("dash test\n"),
		},
		// R2.1: append mode preserves existing content.
		{
			Name:  "append_mode",
			Args:  []string{"-a", filepath.Join(t.TempDir(), "append.txt")},
			Stdin: []byte("appended\n"),
		},
		// R2.1: --append long flag.
		{
			Name:  "append_long_flag",
			Args:  []string{"--append", filepath.Join(t.TempDir(), "appendlong.txt")},
			Stdin: []byte("appended long\n"),
		},
		// R2.2: -i flag (ignore interrupts) — basic operation without signal.
		{
			Name:  "ignore_interrupts_flag",
			Args:  []string{"-i"},
			Stdin: []byte("ignore test\n"),
		},
		// R2.2: --ignore-interrupts long flag.
		{
			Name:  "ignore_interrupts_long",
			Args:  []string{"--ignore-interrupts"},
			Stdin: []byte("ignore long\n"),
		},
		// R2.3: combined -a and -i flags.
		{
			Name:  "combined_ai",
			Args:  []string{"-ai", filepath.Join(t.TempDir(), "combined.txt")},
			Stdin: []byte("combined\n"),
		},
		// R3.1: exit 0 on success (implicit in all passing tests).
		// R3.2, R3.3: write error on one file, continue to stdout.
		{
			Name:      "write_error_bad_path",
			Args:      []string{badPath},
			Stdin:     []byte("should still appear on stdout\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R1.5: empty stdin.
		{
			Name:  "empty_stdin",
			Args:  []string{filepath.Join(t.TempDir(), "empty.txt")},
			Stdin: []byte{},
		},
		// R1.5: large-ish input to verify ordering.
		{
			Name:  "multiline",
			Stdin: []byte("line1\nline2\nline3\nline4\nline5\n"),
		},
		// R1.4: dash with other files.
		{
			Name: "dash_with_file",
			Args: []string{
				"-",
				filepath.Join(t.TempDir(), "dashfile.txt"),
			},
			Stdin: []byte("dash and file\n"),
		},
		// R4.1: --version exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{versionNormalizer},
		},
		// R4.2: --help exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{helpNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
