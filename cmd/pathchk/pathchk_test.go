// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/pathchk.
// Tests cover srd103-pathchk R1.1-R1.4, R2.1-R2.3.
package main

import (
	"os/exec"
	"regexp"
	"strings"
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
	return b
}

// discardOutput normalizes by discarding all output, used when
// output content differs by design (--version, --help).
func discardOutput(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpathchk")
	if err != nil {
		t.Skipf("reference binary gpathchk not in PATH: %v", err)
	}

	// Build boundary-length paths for -p mode testing.
	// POSIX _POSIX_PATH_MAX is 256; paths of 256+ chars fail.
	// Usable limit is 255 chars.
	path255 := strings.Repeat("a/", 127) + "a"  // 254 + 1 = 255 chars
	path256 := strings.Repeat("a/", 127) + "ab" // 254 + 2 = 256 chars

	norms := []testutils.NormalizeFunc{stderrNormalizer}

	tests := []testutils.DiffTest{
		// R1.1: valid path exits 0.
		{
			Name: "valid_simple",
			Args: []string{"validpath"},
		},
		{
			Name: "valid_nested",
			Args: []string{"/usr/bin/sort"},
		},
		{
			Name: "valid_relative",
			Args: []string{"a/b/c"},
		},

		// R1.1: empty path in default mode.
		{
			Name:      "empty_default",
			Args:      []string{""},
			ExitCode:  1,
			Normalize: norms,
		},

		// R1.2: -p portable character set checks.
		{
			Name: "posix_valid",
			Args: []string{"-p", "valid_path.txt"},
		},
		{
			Name:      "posix_invalid_char",
			Args:      []string{"-p", "invalid@path"},
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name:      "posix_space_in_name",
			Args:      []string{"-p", "has space"},
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name:      "posix_long_component",
			Args:      []string{"-p", "abcdefghijklmno"},
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name: "posix_max_component",
			Args: []string{"-p", "abcdefghijklmn"},
		},

		// R1.3: -P extra portability checks (use -- to separate from operands).
		{
			Name:      "extra_leading_dash",
			Args:      []string{"-P", "--", "-filename"},
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name:      "extra_empty",
			Args:      []string{"-P", ""},
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name: "extra_valid",
			Args: []string{"-P", "validname"},
		},
		{
			Name:      "extra_leading_dash_in_component",
			Args:      []string{"-P", "dir/-file"},
			ExitCode:  1,
			Normalize: norms,
		},

		// R1.3 + R1.2: combined -p -P (--portability).
		{
			Name:      "portability_flag",
			Args:      []string{"--portability", "--", "-leadingdash"},
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name:      "combined_pP_invalid_char",
			Args:      []string{"-pP", "bad@name"},
			ExitCode:  1,
			Normalize: norms,
		},

		// R1.4: multiple pathnames.
		{
			Name: "multi_valid",
			Args: []string{"a", "b", "c"},
		},
		{
			Name:      "multi_one_invalid",
			Args:      []string{"-p", "valid", "invalid@"},
			ExitCode:  1,
			Normalize: norms,
		},

		// R1.4: --help and --version.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// No operand error.
		{
			Name:      "no_args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: norms,
		},

		// R2.1: additional exit 0 cases -- various valid paths.
		{
			Name: "exit_0_root",
			Args: []string{"/"},
		},
		{
			Name: "exit_0_dot",
			Args: []string{"."},
		},
		{
			Name: "exit_0_dotdot",
			Args: []string{".."},
		},
		{
			Name: "posix_path_at_limit",
			Args: []string{"-p", path255},
		},
		{
			Name: "extra_root",
			Args: []string{"-P", "/"},
		},
		{
			Name: "extra_dot",
			Args: []string{"-P", "."},
		},
		{
			Name: "posix_trailing_slash",
			Args: []string{"-p", "abc/"},
		},
		{
			Name: "posix_double_slash",
			Args: []string{"-p", "a//b"},
		},
		{
			Name: "posix_nested_max_component",
			Args: []string{"-p", "ab/abcdefghijklmn/cd"},
		},

		// R2.2: exit 1 cases -- path length boundary.
		{
			Name:      "posix_path_over_limit",
			Args:      []string{"-p", path256},
			ExitCode:  1,
			Normalize: norms,
		},

		// R2.2: exit 1 cases -- various nonportable characters.
		{
			Name:      "posix_colon",
			Args:      []string{"-p", "has:colon"},
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name:      "posix_tilde",
			Args:      []string{"-p", "has~tilde"},
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name:      "posix_hash",
			Args:      []string{"-p", "has#hash"},
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name:      "posix_equals",
			Args:      []string{"-p", "has=equals"},
			ExitCode:  1,
			Normalize: norms,
		},

		// R2.2: exit 1 when multiple paths all fail.
		{
			Name:      "exit_1_multi_invalid",
			Args:      []string{"-p", "bad@", "bad!"},
			ExitCode:  1,
			Normalize: norms,
		},

		// R2.2: exit 1 when second path fails (first valid).
		{
			Name:      "exit_1_second_fails",
			Args:      []string{"-p", "good", "bad@"},
			ExitCode:  1,
			Normalize: norms,
		},

		// R2.2: -P edge cases.
		{
			Name:      "extra_bare_dash",
			Args:      []string{"-P", "--", "-"},
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name:      "extra_nested_dash",
			Args:      []string{"-P", "a/-b"},
			ExitCode:  1,
			Normalize: norms,
		},

		// R2.2: -p with empty string.
		{
			Name:      "posix_empty",
			Args:      []string{"-p", ""},
			ExitCode:  1,
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
