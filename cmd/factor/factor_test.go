// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfactor")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name: "single_composite",
			Args: []string{"12"},
		},
		{
			Name: "number_one",
			Args: []string{"1"},
		},
		{
			Name: "prime_number",
			Args: []string{"97"},
		},
		{
			Name: "multiple_arguments",
			Args: []string{"6", "7", "12", "1", "97"},
		},
		{
			Name: "large_composite",
			Args: []string{"1000000"},
		},
		{
			Name: "power_of_two",
			Args: []string{"1024"},
		},
		{
			Name: "prime_squared",
			Args: []string{"49"},
		},
		{
			Name: "two",
			Args: []string{"2"},
		},
		{
			Name: "zero",
			Args: []string{"0"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
