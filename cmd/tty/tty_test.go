// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtty")
	if err != nil {
		t.Skip("reference binary not found")
	}

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?tty`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("tty"))
	})

	tests := []testutils.DiffTest{
		{
			Name:     "pipe-no-args",
			Stdin:    []byte{},
			ExitCode: 1,
		},
		{
			Name:     "pipe-silent-short",
			Args:     []string{"-s"},
			Stdin:    []byte{},
			ExitCode: 1,
		},
		{
			Name:     "pipe-silent-long",
			Args:     []string{"--silent"},
			Stdin:    []byte{},
			ExitCode: 1,
		},
		{
			Name:     "pipe-quiet-long",
			Args:     []string{"--quiet"},
			Stdin:    []byte{},
			ExitCode: 1,
		},
		{
			Name:      "error-unknown-long-flag",
			Args:      []string{"--invalid"},
			Stdin:     []byte{},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "error-unknown-short-flag",
			Args:      []string{"-x"},
			Stdin:     []byte{},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "error-extra-operand",
			Args:      []string{"foo"},
			Stdin:     []byte{},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name:      "error-extra-operand-after-silent",
			Args:      []string{"-s", "foo"},
			Stdin:     []byte{},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
