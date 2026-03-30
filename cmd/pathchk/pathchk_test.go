// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/pathchk (prd103-pathchk R1, R2).
package main

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNameNormalizer replaces the binary name prefix in stderr messages so
// "gpathchk:" and "pathchk:" (possibly with full path) both become "pathchk:".
var stderrNameNormalizer testutils.NormalizeFunc = func() testutils.NormalizeFunc {
	re := regexp.MustCompile(`(?m)^[^\s:]*(?:gpathchk|pathchk):`)
	return func(data []byte) []byte {
		return re.ReplaceAll(data, []byte("pathchk:"))
	}
}()

// stderrTryLineNormalizer strips the "Try ... --help" line GNU appends.
var stderrTryLineNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	re := regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)
	return re.ReplaceAll(data, nil)
}

// stderrPortableNormalizer normalizes "nonportable" (Go) to "non-portable" (GNU).
var stderrPortableNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	return []byte(strings.ReplaceAll(string(data), "nonportable", "non-portable"))
}

// stderrLimitNormalizer normalizes Go's "limit N exceeded by length M of
// file name component 'X'" to GNU's default-mode format "X: File name too long".
// Both binaries use the "limit ... exceeded" format for -p mode, so applying
// this globally is safe (both get changed identically).
var stderrLimitNormalizer testutils.NormalizeFunc = func() testutils.NormalizeFunc {
	re := regexp.MustCompile(
		`limit \d+ exceeded by length \d+ of file name component '([^']*)'`,
	)
	return func(data []byte) []byte {
		return re.ReplaceAll(data, []byte("$1: File name too long"))
	}
}()

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpathchk")
	if err != nil {
		t.Skipf("reference binary gpathchk not in PATH: %v", err)
	}

	normalizers := []testutils.NormalizeFunc{
		stderrNameNormalizer,
		stderrTryLineNormalizer,
		stderrPortableNormalizer,
		stderrLimitNormalizer,
	}

	// R1.1: component exceeding NAME_MAX (255 on macOS/Linux).
	longComponent := strings.Repeat("a", 256)

	// R1.2: component exceeding _POSIX_NAME_MAX (14).
	posixLongComponent := strings.Repeat("a", 15)

	// R1.2: path exceeding _POSIX_PATH_MAX (256).
	posixLongPath := strings.Repeat("a", 257)

	tests := []testutils.DiffTest{
		// --- R1.1: Default mode pathname checking ---

		// R1.1, R2.1: valid pathname exits 0
		{
			Name:      "R1.1_valid_path",
			Args:      []string{"validpath"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R1.1: single-character path
		{
			Name:      "R1.1_single_char",
			Args:      []string{"x"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R1.1: component exceeding NAME_MAX, exit 1
		{
			Name:      "R1.1_component_too_long",
			Args:      []string{longComponent},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R1.1: absolute path to existing directory
		{
			Name:      "R1.1_absolute_existing",
			Args:      []string{"/tmp"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},

		// --- R1.2: POSIX portability (-p) checks ---

		// R1.2: non-portable character '@', exit 1
		{
			Name:      "R1.2_nonportable_at",
			Args:      []string{"-p", "invalid@path"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R1.2: non-portable character space, exit 1
		{
			Name:      "R1.2_nonportable_space",
			Args:      []string{"-p", "has space"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R1.2: component exceeding _POSIX_NAME_MAX (14), exit 1
		{
			Name:      "R1.2_posix_component_too_long",
			Args:      []string{"-p", posixLongComponent},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R1.2: path exceeding _POSIX_PATH_MAX (256), exit 1
		{
			Name:      "R1.2_posix_path_too_long",
			Args:      []string{"-p", posixLongPath},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R1.2: valid portable path (14 chars max component), exit 0
		{
			Name:      "R1.2_valid_portable",
			Args:      []string{"-p", "valid-name.txt"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R1.2: empty string argument, exit 1
		{
			Name:      "R1.2_empty_string",
			Args:      []string{"-p", ""},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},

		// --- R1.3: -P leading hyphen check ---

		// R1.3: leading hyphen in component, exit 1
		{
			Name:      "R1.3_leading_hyphen",
			Args:      []string{"-P", "--", "-filename"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R1.3: leading hyphen in nested component, exit 1
		{
			Name:      "R1.3_leading_hyphen_nested",
			Args:      []string{"-P", "dir/-file"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R1.3: no leading hyphen, exit 0
		{
			Name:      "R1.3_no_leading_hyphen",
			Args:      []string{"-P", "safe_name"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R1.3: -P with empty string, exit 1
		{
			Name:      "R1.3_empty_string",
			Args:      []string{"-P", ""},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},

		// --- R1.4: Multiple pathname arguments ---

		// R1.4: multiple valid pathnames, exit 0
		{
			Name:      "R1.4_multiple_valid",
			Args:      []string{"path1", "path2", "path3"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R1.4: mix of valid and invalid with -p, exit 1
		{
			Name:      "R1.4_mixed_valid_invalid",
			Args:      []string{"-p", "valid", "bad@name"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},

		// --- R2.1, R2.2: Combined -p -P / --portability ---

		// R2.1: --portability with non-portable char, exit 1
		{
			Name:      "R2.1_portability_nonportable",
			Args:      []string{"--portability", "bad@name"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R2.2: combined -p -P with leading hyphen via --, exit 1
		{
			Name:      "R2.2_combined_pP_leading_hyphen",
			Args:      []string{"-p", "-P", "--", "-name"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R2.2: --portability with leading hyphen via --, exit 1
		{
			Name:      "R2.2_portability_leading_hyphen",
			Args:      []string{"--portability", "--", "-name"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R2.1: --portability valid path, exit 0
		{
			Name:      "R2.1_portability_valid",
			Args:      []string{"--portability", "goodname"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},

		// --- R2.1, R2.2: Exit code verification ---

		// R2.1: all valid paths exit 0
		{
			Name:      "R2.1_exit_0_all_valid",
			Args:      []string{"a", "b", "c"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R2.2: any invalid path exits 1
		{
			Name:      "R2.2_exit_1_any_invalid",
			Args:      []string{"-p", "ok", "bad@"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
