// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer strips the program name prefix (seq:/gseq:) and removes
// the "Try ... --help" hint line so error stderr can be compared.
var stderrNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	// Remove "Try '...' for more information.\n" lines.
	reTry := regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)
	b = reTry.ReplaceAll(b, nil)
	// Normalize program name prefix to "seq:".
	b = bytes.Replace(b, []byte("gseq:"), []byte("seq:"), -1)
	return b
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gseq")
	if err != nil {
		t.Skipf("reference binary gseq not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name: "single_arg",
			Args: []string{"5"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "two_args",
			Args: []string{"2", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "three_args",
			Args: []string{"1", "2", "10"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "descending",
			Args: []string{"5", "-1", "1"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "first_equals_last",
			Args: []string{"3", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "empty_ascending",
			Args: []string{"5", "1"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "floating_point",
			Args: []string{"0.1", "0.1", "0.5"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "custom_separator",
			Args: []string{"-s", ", ", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "equal_width",
			Args: []string{"-w", "8", "12"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "format_string",
			Args: []string{"-f", "%.2f", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name:      "zero_step_error",
			Args:      []string{"1", "0", "5"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		{
			Name:      "invalid_format_error",
			Args:      []string{"-f", "%s", "1", "3"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		{
			Name: "negative_step_descending",
			Args: []string{"10", "-2", "1"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "single_number_1",
			Args: []string{"1"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "large_equal_width",
			Args: []string{"-w", "1", "100"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "negative_to_positive",
			Args: []string{"-5", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "format_e",
			Args: []string{"-f", "%e", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "format_g",
			Args: []string{"-f", "%g", "1", "3"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "separator_tab",
			Args: []string{"-s", "\t", "1", "5"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "equal_width_negative",
			Args: []string{"-w", "-5", "5"},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
