// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd033-sha512sum R1.1, R1.2, R1.3, R1.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// setupFileDir creates a temp directory with the given files and returns the path.
func setupFileDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		writeTestFile(t, dir, name, content)
	}
	return dir
}

// writeTestFile writes a file in the given directory.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
}

// errLinePattern matches error lines from both gsha512sum and sha512sum.
var errLinePattern = regexp.MustCompile(`(?:g?sha512sum): [^\n]*\n`)

// stderrNormalizer replaces stderr error lines with a fixed placeholder
// to account for binary name and message format differences.
func stderrNormalizer(data []byte) []byte {
	return errLinePattern.ReplaceAll(data, []byte("sha512sum: <ERROR>\n"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha512sum")
	if err != nil {
		t.Skipf("reference binary gsha512sum not in PATH: %v", err)
	}

	dirSingle := setupFileDir(t, map[string]string{
		"input.txt": "hello world\n",
	})
	dirTag := setupFileDir(t, map[string]string{
		"input.txt": "test data\n",
	})
	dirMulti := setupFileDir(t, map[string]string{
		"a.txt": "aaa\n",
		"b.txt": "bbb\n",
	})
	dirMissing := setupFileDir(t, map[string]string{
		"exists.txt": "data\n",
	})

	tests := []testutils.DiffTest{
		// R1.1: Compute digest of a file in text mode (default).
		{
			Name:    "file_text_mode",
			Args:    []string{"input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.1: Compute digest of a file in binary mode.
		{
			Name:    "file_binary_mode",
			Args:    []string{"-b", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.2: Read from stdin when no file arguments given.
		{
			Name:  "stdin_no_args",
			Stdin: []byte("abc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: Read from stdin when "-" is given as filename.
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("abc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: BSD tag output format.
		{
			Name:    "tag_format",
			Args:    []string{"--tag", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirTag,
		},
		// R1.4: Error on nonexistent file, exit 1, continue remaining.
		{
			Name:      "missing_file_continues",
			Args:      []string{"nonexistent.txt", "exists.txt"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			WorkDir:   dirMissing,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R1.1: Multiple files.
		{
			Name:    "multiple_files",
			Args:    []string{"a.txt", "b.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirMulti,
		},
		// R1.2: Empty stdin.
		{
			Name:  "empty_stdin",
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: --binary long flag produces "HASH *FILENAME" format.
		{
			Name:    "binary_flag_long",
			Args:    []string{"--binary", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.1: -b with stdin.
		{
			Name:  "binary_flag_stdin",
			Args:  []string{"-b"},
			Stdin: []byte("abc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: -t flag is default text mode, "HASH  FILENAME".
		{
			Name:    "text_flag_explicit",
			Args:    []string{"-t", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.1: -t overrides earlier -b.
		{
			Name:    "text_overrides_binary",
			Args:    []string{"-b", "-t", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.1: -b overrides earlier -t.
		{
			Name:    "binary_overrides_text",
			Args:    []string{"-t", "-b", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.3: --tag with -b produces BSD format (tag takes precedence).
		{
			Name:    "tag_with_binary",
			Args:    []string{"--tag", "-b", "input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.1: -b with multiple files.
		{
			Name:    "binary_multiple_files",
			Args:    []string{"-b", "a.txt", "b.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirMulti,
		},
		// R1.4: Exit 0 when all files processed successfully.
		{
			Name:    "exit_zero_single_file",
			Args:    []string{"input.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingle,
		},
		// R1.4: Exit 1 when a file cannot be opened.
		{
			Name:      "exit_one_missing_file",
			Args:      []string{"no_such_file.txt"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			WorkDir:   dirSingle,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
