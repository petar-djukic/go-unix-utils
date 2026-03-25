// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/pathchk against gpathchk.
// Implements prd103-pathchk R2.1, R2.2, R2.3.
package main

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// progNameRe matches the pathchk binary name with optional path prefix.
var progNameRe = regexp.MustCompile(`(?:/[\w/._-]+/)?g?pathchk`)

// limitRe normalizes the numeric limit value in error messages. GNU pathchk
// reports PATH_MAX as 255 (excluding null terminator) while the Go
// implementation uses 256. Normalizing the number avoids a false divergence.
var limitRe = regexp.MustCompile(`limit \d+`)

// normProg normalizes the program name in output so the Go binary name
// and the reference binary name (including full paths) compare equal.
func normProg(b []byte) []byte {
	return progNameRe.ReplaceAll(b, []byte("PROG"))
}

// normWording normalizes the wording difference between "nonportable"
// (Go implementation) and "non-portable" (GNU reference).
func normWording(b []byte) []byte {
	return []byte(strings.ReplaceAll(string(b), "nonportable", "non-portable"))
}

// normLimit normalizes limit values so PATH_MAX differences (255 vs 256)
// do not cause false divergences.
func normLimit(b []byte) []byte {
	return limitRe.ReplaceAll(b, []byte("limit N"))
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpathchk")
	if err != nil {
		t.Skipf("reference binary gpathchk not in PATH: %v", err)
	}

	norms := []testutils.NormalizeFunc{normProg, normWording, normLimit}

	tests := []testutils.DiffTest{
		// ── R2.1: default mode — valid paths ──
		{Name: "default_valid_simple", Args: []string{"hello"}, Normalize: norms},
		{Name: "default_valid_slash", Args: []string{"/tmp"}, Normalize: norms},
		{Name: "default_valid_dot", Args: []string{"."}, Normalize: norms},
		{Name: "default_valid_dotdot", Args: []string{".."}, Normalize: norms},
		{Name: "default_valid_multi", Args: []string{"foo", "bar", "baz"}, Normalize: norms},

		// ── R2.2: -p mode — valid portable paths ──
		{Name: "p_valid_simple", Args: []string{"-p", "hello"}, Normalize: norms},
		{Name: "p_valid_path", Args: []string{"-p", "a/b/c"}, Normalize: norms},
		{Name: "p_valid_dot", Args: []string{"-p", "."}, Normalize: norms},
		{Name: "p_valid_underscore", Args: []string{"-p", "foo_bar-baz.txt"}, Normalize: norms},
		{Name: "p_valid_multi", Args: []string{"-p", "a", "b", "c"}, Normalize: norms},

		// R2.2: -p mode — component at exactly POSIX NAME_MAX (14)
		{Name: "p_component_exactly_14", Args: []string{
			"-p", "abcdefghijklmn",
		}, Normalize: norms},

		// R2.2: -p mode — component exceeding POSIX NAME_MAX (14)
		{Name: "p_overlong_component", Args: []string{
			"-p", "abcdefghijklmno",
		}, Normalize: norms},

		// R2.2: -p mode — path exceeding POSIX PATH_MAX (256)
		{Name: "p_overlong_path", Args: []string{
			"-p", strings.Repeat("a/", 129),
		}, Normalize: norms},

		// R2.2: -p mode — non-portable characters
		{Name: "p_nonportable_at", Args: []string{"-p", "file@name"}, Normalize: norms},
		{Name: "p_nonportable_space", Args: []string{"-p", "file name"}, Normalize: norms},
		{Name: "p_nonportable_colon", Args: []string{"-p", "file:name"}, Normalize: norms},
		{Name: "p_nonportable_tilde", Args: []string{"-p", "~user"}, Normalize: norms},

		// R2.2: --portability long form
		{Name: "portability_long_flag", Args: []string{
			"--portability", "file@name",
		}, Normalize: norms},

		// R2.2: -p mode — multiple args, mix valid and invalid
		{Name: "p_multiple_mixed", Args: []string{
			"-p", "valid", "file@bad",
		}, Normalize: norms},

		// ── R2.3: -P mode — valid paths ──
		{Name: "P_valid", Args: []string{"-P", "hello"}, Normalize: norms},

		// R2.3: -P mode — empty path argument
		{Name: "P_empty_string", Args: []string{"-P", ""}, Normalize: norms},

		// R2.3: -P mode — leading hyphen (use -- to pass as operand)
		{Name: "P_leading_hyphen", Args: []string{
			"-P", "--", "-file",
		}, Normalize: norms},

		// ── R2.3: combined -p -P mode ──
		{Name: "pP_valid", Args: []string{"-p", "-P", "hello"}, Normalize: norms},
		{Name: "pP_combined_flag", Args: []string{"-pP", "hello"}, Normalize: norms},

		// R2.3: combined -pP — leading hyphen
		{Name: "pP_leading_hyphen", Args: []string{
			"-pP", "--", "-file",
		}, Normalize: norms},

		// R2.3: combined -pP — leading hyphen in sub-component
		{Name: "pP_leading_hyphen_subdir", Args: []string{
			"-pP", "a/-file",
		}, Normalize: norms},

		// R2.3: combined -pP — overlong component
		{Name: "pP_overlong_component", Args: []string{
			"-pP", "abcdefghijklmno",
		}, Normalize: norms},

		// ── Edge cases ──
		{Name: "no_args", Args: []string{}, Normalize: norms},
		{Name: "double_dash_separator", Args: []string{"--", "hello"}, Normalize: norms},
		{Name: "single_hyphen_operand", Args: []string{"-p", "-"}, Normalize: norms},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
