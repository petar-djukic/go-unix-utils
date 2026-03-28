// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/uname against GNU guname reference binary.
// Implements prd044 R4.1-R4.3: differential tests covering all flags,
// combinations, error cases, --version, and --help.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// progNameRe matches the binary path/name at the start of error lines
// and inside single-quoted references like "Try '... --help'".
// Handles full paths (/opt/homebrew/bin/uname), guname, and uname.
var progNameRe = regexp.MustCompile(`(?:/[\w/.-]+/)?g?uname`)

// normalizeProgramName replaces any form of the binary name (full path,
// guname, uname) with "uname" so error messages from the reference
// binary match our hardcoded program name.
func normalizeProgramName(b []byte) []byte {
	return progNameRe.ReplaceAll(b, []byte("uname"))
}

// normalizeContent clears output to compare only exit codes.
// Used for --version and --help where content intentionally differs
// between our implementation and the GNU reference binary.
func normalizeContent(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("guname")
	if err != nil {
		t.Skipf("reference binary guname not in PATH: %v", err)
	}

	errNorm := []testutils.NormalizeFunc{normalizeProgramName}
	contentNorm := []testutils.NormalizeFunc{normalizeContent}

	tests := []testutils.DiffTest{
		// R1.1: default (no flags) prints kernel name.
		{Name: "default_no_flags"},
		// R1.2-R1.9: individual flags.
		{Name: "flag_s", Args: []string{"-s"}},
		{Name: "flag_n", Args: []string{"-n"}},
		{Name: "flag_r", Args: []string{"-r"}},
		{Name: "flag_v", Args: []string{"-v"}},
		{Name: "flag_m", Args: []string{"-m"}},
		{Name: "flag_p", Args: []string{"-p"}},
		{Name: "flag_i", Args: []string{"-i"}},
		{Name: "flag_o", Args: []string{"-o"}},
		// R2.1: -a combined output in canonical order.
		{Name: "flag_a", Args: []string{"-a"}},
		// R2.2: multi-flag combinations in canonical order.
		{Name: "flags_sn", Args: []string{"-sn"}},
		{Name: "flags_separate_s_r", Args: []string{"-s", "-r"}},
		{Name: "flags_snrvm", Args: []string{"-snrvm"}},
		{Name: "flags_m_o", Args: []string{"-m", "-o"}},
		{Name: "flags_p_i", Args: []string{"-p", "-i"}},
		// R3.1: extra operand produces error on stderr, exit 1.
		{
			Name:      "extra_operand",
			Args:      []string{"extra"},
			Normalize: errNorm,
		},
		// R3.2: invalid short flag produces error on stderr, exit 1.
		{
			Name:      "invalid_flag_z",
			Args:      []string{"-z"},
			Normalize: errNorm,
		},
		// R3.2: unrecognized long option.
		{
			Name:      "unrecognized_long_option",
			Args:      []string{"--foo"},
			Normalize: errNorm,
		},
		// R4.1: --version exits 0 (content differs, compare exit code only).
		{
			Name:      "version",
			Args:      []string{"--version"},
			Normalize: contentNorm,
		},
		// R4.1: --help exits 0 (content differs, compare exit code only).
		{
			Name:      "help",
			Args:      []string{"--help"},
			Normalize: contentNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
