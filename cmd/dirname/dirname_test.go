// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/dirname.
// Tests cover srd016-dirname R3.1, R3.2, R3.3, R4.1-R4.3.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgRe matches the program name/path prefix before a colon at line start.
var stderrProgRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// stderrTryRe matches the quoted program reference in Try hint lines.
var stderrTryRe = regexp.MustCompile(`'[^']*--help'`)

// stderrNormalizer normalizes program name differences in error messages.
// R3.2/R4.3: replaces binary paths with "PROG" so error message structure
// can be compared between Go and GNU binaries.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = stderrTryRe.ReplaceAll(b, []byte("'PROG --help'"))
	return b
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdirname")
	if err != nil {
		t.Skipf("reference binary gdirname not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: simple path strips last component.
		{Name: "simple_path", Args: []string{"/usr/bin/sort"}},
		// R1.1: nested path.
		{Name: "nested_path", Args: []string{"/a/b/c"}},
		// R1.2: no slash returns dot.
		{Name: "no_slash", Args: []string{"stdio.h"}},
		// R1.2: single filename with no directory.
		{Name: "plain_filename", Args: []string{"file.txt"}},
		// R1.3: root path returns root.
		{Name: "root", Args: []string{"/"}},
		// R1.3: multiple slashes returns root.
		{Name: "multiple_slashes", Args: []string{"///"}},
		// R1.1+R1.4: trailing slashes stripped before extraction.
		{Name: "trailing_slash", Args: []string{"/usr/bin/"}},
		// R1.4: trailing slashes on result stripped.
		{Name: "trailing_slashes_nested", Args: []string{"/usr///bin///sort"}},
		// R1.2: dot path.
		{Name: "dot", Args: []string{"."}},
		// R1.2: double-dot path.
		{Name: "double_dot", Args: []string{".."}},
		// R1.5: multiple arguments produce one result per line.
		{Name: "multiple_args", Args: []string{"/usr/bin", "/etc/passwd"}},
		// R1.1: relative path with directory.
		{Name: "relative_path", Args: []string{"dir/file"}},
		// R1.3+R1.4: single slash in middle.
		{Name: "root_file", Args: []string{"/file"}},
		// R1.5: multiple arguments with mixed types.
		{Name: "multiple_mixed", Args: []string{"/usr/lib", "file.txt", "/", "a/b/c"}},
		// R2.1: -z flag outputs NUL instead of newline.
		{Name: "zero_single", Args: []string{"-z", "/usr/bin/sort"}},
		// R2.2: -z with multiple arguments, NUL-terminated in order.
		{Name: "zero_multiple", Args: []string{"-z", "/usr/bin", "/etc/passwd"}},
		// R2.1: --zero long flag form.
		{Name: "zero_long_flag", Args: []string{"--zero", "/usr/lib"}},
		// R2.2: -z with various path types.
		{Name: "zero_mixed", Args: []string{"-z", "/", "file.txt", "a/b/c"}},

		// R3.3: edge case — empty string argument.
		{Name: "empty_string", Args: []string{""}},
		// R3.3: edge case — multiple trailing slashes on dir portion.
		{Name: "dir_trailing_slashes", Args: []string{"/usr///lib"}},
		// R3.3: edge case — path that is just a filename with dots.
		{Name: "dotfile", Args: []string{".bashrc"}},
		// R3.3: edge case — relative path with dot-dot.
		{Name: "relative_dotdot", Args: []string{"../foo/bar"}},
		// R3.3: edge case — path ending with dot-dot.
		{Name: "path_ending_dotdot", Args: []string{"/foo/.."}},

		// R3.2: error case — no arguments exits 1 with stderr message.
		{
			Name:      "no_args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
