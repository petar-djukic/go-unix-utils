// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sha512sum.
//
// Implements: prd033-sha512sum R1.1–R1.4
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const binGsha512sum = "gsha512sum"

// sha512sumErrRe matches sha512sum/gsha512sum error lines and normalizes the program
// name and error format differences between GNU and Go implementations.
var sha512sumErrRe = regexp.MustCompile(`(?m)^g?sha512sum: .+?: .+$`)

// normalizeSha512sumErrors replaces sha512sum error lines with a canonical form so
// that minor wording differences between GNU and Go do not cause false failures.
func normalizeSha512sumErrors(b []byte) []byte {
	return sha512sumErrRe.ReplaceAll(b, []byte("PROG: FILE: ERROR"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGsha512sum)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGsha512sum, err)
	}

	// Create temp files for file-based test cases.
	dir := t.TempDir()

	// Known-content file for digest tests.
	helloFile := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(helloFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("writing hello.txt: %v", err)
	}

	// Second file for multi-file tests.
	worldFile := filepath.Join(dir, "world.txt")
	if err := os.WriteFile(worldFile, []byte("world\n"), 0o644); err != nil {
		t.Fatalf("writing world.txt: %v", err)
	}

	// Empty file.
	emptyFile := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatalf("writing empty.txt: %v", err)
	}

	// Non-existent file for error tests.
	missing := filepath.Join(dir, "nonexistent.txt")

	// Unreadable file for permission-denied tests.
	unreadable := filepath.Join(dir, "noperm.txt")
	if err := os.WriteFile(unreadable, []byte("secret\n"), 0o000); err != nil {
		t.Fatalf("writing noperm.txt: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: compute SHA-512 of a single file in text mode (default).
		{
			Name: "r1.1_single_file_text_mode",
			Args: []string{helloFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: compute SHA-512 of multiple files.
		{
			Name: "r1.1_multiple_files",
			Args: []string{helloFile, worldFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: binary mode output format "HASH *FILENAME".
		{
			Name: "r1.1_binary_mode",
			Args: []string{"-b", helloFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: empty file produces valid digest.
		{
			Name: "r1.1_empty_file",
			Args: []string{emptyFile},
			Env:  []string{"LC_ALL=C"},
		},

		// R1.2: stdin when no file arguments.
		{
			Name:  "r1.2_stdin_no_args",
			Stdin: []byte("abc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: stdin with empty input.
		{
			Name:  "r1.2_stdin_empty",
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: "-" means stdin.
		{
			Name:  "r1.2_dash_means_stdin",
			Args:  []string{"-"},
			Stdin: []byte("test input\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: stdin with newline content.
		{
			Name:  "r1.2_stdin_with_newline",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},

		// R1.3: --tag BSD-style output.
		{
			Name: "r1.3_tag_mode_file",
			Args: []string{"--tag", helloFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: --tag with stdin.
		{
			Name:  "r1.3_tag_mode_stdin",
			Args:  []string{"--tag"},
			Stdin: []byte("abc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: --tag with multiple files.
		{
			Name: "r1.3_tag_mode_multiple_files",
			Args: []string{"--tag", helloFile, worldFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: --tag with -b (binary flag has no effect on tag format).
		{
			Name: "r1.3_tag_with_binary",
			Args: []string{"--tag", "-b", helloFile},
			Env:  []string{"LC_ALL=C"},
		},

		// R1.4: exit 1 on missing file.
		{
			Name:      "r1.4_missing_file",
			Args:      []string{missing},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSha512sumErrors},
		},
		// R1.4: missing file then existing file — exit 1, continues processing.
		{
			Name:      "r1.4_missing_then_existing",
			Args:      []string{missing, helloFile},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSha512sumErrors},
		},
		// R1.4: existing file then missing file — exit 1.
		{
			Name:      "r1.4_existing_then_missing",
			Args:      []string{helloFile, missing},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSha512sumErrors},
		},
		// R1.4: permission denied — exit 1.
		{
			Name:      "r1.4_permission_denied",
			Args:      []string{unreadable},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSha512sumErrors},
		},
		// R1.4: multiple missing files — all errors, exit 1.
		{
			Name:      "r1.4_multiple_missing",
			Args:      []string{missing, filepath.Join(dir, "also_missing.txt")},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSha512sumErrors},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffBinaryTextMode runs differential tests for binary/text mode flags.
func TestDiffBinaryTextMode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGsha512sum)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGsha512sum, err)
	}

	dir := t.TempDir()

	dataFile := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(dataFile, []byte("binary text mode test\n"), 0o644); err != nil {
		t.Fatalf("writing data.txt: %v", err)
	}

	secondFile := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(secondFile, []byte("second\n"), 0o644); err != nil {
		t.Fatalf("writing second.txt: %v", err)
	}

	tests := []testutils.DiffTest{
		// -b produces "HASH *FILENAME".
		{
			Name: "binary_flag_short",
			Args: []string{"-b", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// --binary long form.
		{
			Name: "binary_flag_long",
			Args: []string{"--binary", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// -b with multiple files.
		{
			Name: "binary_multiple_files",
			Args: []string{"-b", dataFile, secondFile},
			Env:  []string{"LC_ALL=C"},
		},
		// -b with stdin.
		{
			Name:  "binary_stdin",
			Args:  []string{"-b"},
			Stdin: []byte("stdin binary\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// -t (text mode, default) produces "HASH  FILENAME" (two spaces).
		{
			Name: "text_flag_short",
			Args: []string{"-t", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// --text long form.
		{
			Name: "text_flag_long",
			Args: []string{"--text", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// Default (no flag) is text mode.
		{
			Name: "default_text_mode",
			Args: []string{dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// --tag with -b uses BSD tag format (mode has no effect).
		{
			Name: "tag_with_binary",
			Args: []string{"--tag", "-b", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// --tag without -b also uses BSD tag format.
		{
			Name: "tag_without_binary",
			Args: []string{"--tag", dataFile},
			Env:  []string{"LC_ALL=C"},
		},
		// --tag with multiple files.
		{
			Name: "tag_multiple_files",
			Args: []string{"--tag", dataFile, secondFile},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffVersionHelpErrors runs differential tests for --version, --help, and
// invalid option handling.
func TestDiffVersionHelpErrors(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGsha512sum)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGsha512sum, err)
	}

	dir := t.TempDir()
	missing := filepath.Join(dir, "nonexistent.txt")

	// normalizeNonEmpty replaces all non-empty output with a fixed marker so that
	// implementation-specific text (version strings, help text) does not cause
	// false failures. Only the exit code is meaningfully compared.
	normalizeNonEmpty := func(b []byte) []byte {
		if len(b) == 0 {
			return b
		}
		return []byte("OUTPUT\n")
	}

	tests := []testutils.DiffTest{
		// --version prints version info and exits 0.
		{
			Name:      "version",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
		// --help prints usage info and exits 0.
		{
			Name:      "help",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
		// Invalid option exits non-zero with error on stderr.
		{
			Name:      "invalid_option",
			Args:      []string{"--invalid-option-xyz"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
		// Nonexistent file prints error to stderr and exits non-zero.
		{
			Name:      "nonexistent_file",
			Args:      []string{missing},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeSha512sumErrors},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
