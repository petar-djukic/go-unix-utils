// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinName is the Homebrew GNU reference binary for seq.
const refBinName = "gseq"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	errNorm := normalizeErrPrefix()

	tests := []testutils.DiffTest{
		// R1.1: Single argument — seq LAST.
		{
			Name: "single_arg_5",
			Args: []string{"5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: Two arguments — seq FIRST LAST.
		{
			Name: "two_args_2_5",
			Args: []string{"2", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1, R1.2: Three arguments — seq FIRST STEP LAST.
		{
			Name: "three_args_1_2_10",
			Args: []string{"1", "2", "10"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: Descending sequence.
		{
			Name: "descending_5_to_1",
			Args: []string{"5", "-1", "1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: FIRST equals LAST.
		{
			Name: "first_equals_last",
			Args: []string{"3", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Empty sequence (FIRST > LAST, positive step).
		{
			Name: "empty_ascending",
			Args: []string{"5", "1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5, R4.2: Zero step error.
		{
			Name:      "zero_step_error",
			Args:      []string{"1", "0", "5"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.2: Custom separator.
		{
			Name: "custom_separator",
			Args: []string{"-s", ", ", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: Floating-point sequence.
		{
			Name: "floating_point",
			Args: []string{"0.1", "0.1", "0.5"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: Format string.
		{
			Name: "format_f_2",
			Args: []string{"-f", "%.2f", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2, R4.2: Invalid format error.
		{
			Name:      "invalid_format_s",
			Args:      []string{"-f", "%s", "1", "3"},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R3.3: Equal width.
		{
			Name: "equal_width_8_12",
			Args: []string{"-w", "8", "12"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: -f overrides -w.
		{
			Name: "format_overrides_width",
			Args: []string{"-f", "%.2f", "-w", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: seq 1 (single number to 1).
		{
			Name: "single_arg_1",
			Args: []string{"1"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: Integer format for integer args.
		{
			Name: "integer_format",
			Args: []string{"1", "1", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// Descending floating point.
		{
			Name: "descending_float",
			Args: []string{"1.0", "-0.5", "-1.0"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: Equal width with leading zeros, wider range.
		{
			Name: "equal_width_1_10",
			Args: []string{"-w", "1", "10"},
			Env:  []string{"LC_ALL=C"},
		},
		// Large step that overshoots.
		{
			Name: "step_overshoots",
			Args: []string{"1", "10", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		// Negative first to positive last.
		{
			Name: "negative_to_positive",
			Args: []string{"-2", "2"},
			Env:  []string{"LC_ALL=C"},
		},
		// Format with %e.
		{
			Name: "format_e",
			Args: []string{"-f", "%e", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// Format with %g.
		{
			Name: "format_g",
			Args: []string{"-f", "%g", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		// seq 1 0.5 3 (float step with integer endpoints).
		{
			Name: "float_step_int_endpoints",
			Args: []string{"1", "0.5", "3"},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeErrPrefix returns a NormalizeFunc that normalizes binary name
// prefixes in stderr (e.g. "gseq:" or "/tmp/.../seq:") to "seq:".
func normalizeErrPrefix() testutils.NormalizeFunc {
	re := regexp.MustCompile(`(?m)^[^\s:]*seq[^\s:]*:`)
	return func(b []byte) []byte {
		return re.ReplaceAll(b, []byte("seq:"))
	}
}
