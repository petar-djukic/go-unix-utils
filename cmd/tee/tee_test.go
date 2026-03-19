// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd017-tee R4.1–R4.3: compare Go tee against
// gtee reference binary for stdout, file content, and exit codes.
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameNormalizer replaces the reference binary name and path in
// stderr so that "gtee" and "/opt/.../gtee" both become "tee".
var binaryNameNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	re := regexp.MustCompile(`[^\s']*g?tee`)
	return re.ReplaceAll(data, []byte("tee"))
}

// errorCaseNormalizer lowercases stderr so that platform-specific error
// message casing (e.g., "No such file" vs "no such file") does not cause
// false divergence.
var errorCaseNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	return bytes.ToLower(data)
}

// versionNormalizer replaces all version output with a fixed string so
// that GNU's multi-line copyright block and Go's single-line version match.
var versionNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	if len(data) > 0 {
		return []byte("VERSION OUTPUT")
	}
	return data
}

// helpNormalizer replaces all help output with a fixed string so that
// structural differences between GNU and Go help text don't cause failures.
var helpNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	if len(data) > 0 {
		return []byte("HELP OUTPUT")
	}
	return data
}

// TestDiff runs differential tests comparing Go tee against gtee.
// Implements prd017-tee R4.1, R4.2.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}
	errNorm := []testutils.NormalizeFunc{binaryNameNormalizer, errorCaseNormalizer}
	tests := []testutils.DiffTest{
		// R4.2: passthrough mode (no files) — stdin to stdout only.
		{
			Name:  "passthrough_no_files",
			Args:  []string{},
			Stdin: []byte("hello\nworld\n"),
		},
		// R4.1, R4.3: single output file — file content must match stdout.
		{
			Name:  "single_file",
			Args:  []string{"out.txt"},
			Stdin: []byte("hello\nworld\n"),
			ExpectedFiles: map[string][]byte{
				"out.txt": []byte("hello\nworld\n"),
			},
		},
		// R4.2: multiple output files.
		{
			Name:  "multiple_files",
			Args:  []string{"a.txt", "b.txt"},
			Stdin: []byte("line1\nline2\n"),
			ExpectedFiles: map[string][]byte{
				"a.txt": []byte("line1\nline2\n"),
				"b.txt": []byte("line1\nline2\n"),
			},
		},
		// R4.2: empty stdin produces empty file.
		{
			Name:  "empty_stdin",
			Args:  []string{"out.txt"},
			Stdin: []byte{},
			ExpectedFiles: map[string][]byte{
				"out.txt": {},
			},
		},
		// R4.2: append mode (-a) with new file — creates file.
		{
			Name:  "append_mode_new_file",
			Args:  []string{"-a", "out.txt"},
			Stdin: []byte("appended\n"),
		},
		// R4.2: combined -a and -i flags.
		{
			Name:  "combined_ai_flags",
			Args:  []string{"-ai", "out.txt"},
			Stdin: []byte("data\n"),
		},
		// R2.2: --ignore-interrupts flag.
		{
			Name:  "ignore_interrupts_flag",
			Args:  []string{"-i", "out.txt"},
			Stdin: []byte("data\n"),
			ExpectedFiles: map[string][]byte{
				"out.txt": []byte("data\n"),
			},
		},
		// R1.4: dash "-" as additional stdout reference.
		{
			Name:  "dash_stdout_ref",
			Args:  []string{"-"},
			Stdin: []byte("data\n"),
		},
		// R4.2: write error — non-existent directory path.
		{
			Name:      "write_error_nonexistent_dir",
			Args:      []string{"/nonexistent_path_for_tee_test/out.txt"},
			Stdin:     []byte("data\n"),
			ExitCode:  1,
			Normalize: errNorm,
		},
		// Long flag --append form.
		{
			Name:  "long_append_flag",
			Args:  []string{"--append", "out.txt"},
			Stdin: []byte("data\n"),
		},
		// Long flag --ignore-interrupts form.
		{
			Name:  "long_ignore_interrupts_flag",
			Args:  []string{"--ignore-interrupts", "out.txt"},
			Stdin: []byte("data\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestVersion verifies that --version prints version info to stdout
// and exits 0. Implements prd017-tee R4.3.
func TestVersion(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}
	verNorm := []testutils.NormalizeFunc{versionNormalizer}
	tests := []testutils.DiffTest{
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: verNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelp verifies that --help prints usage to stdout and exits 0.
// Implements prd017-tee R4.3.
func TestHelp(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}
	helpNorm := []testutils.NormalizeFunc{helpNormalizer}
	tests := []testutils.DiffTest{
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: helpNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
