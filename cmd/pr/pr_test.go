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

// prDateRe matches YYYY-MM-DD HH:MM timestamps in pr header lines.
var prDateRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}`)

// normalizePrDate replaces date timestamps with a fixed string so
// differential tests are not sensitive to wall-clock time.
func normalizePrDate(b []byte) []byte {
	return prDateRe.ReplaceAll(b, []byte("YYYY-MM-DD HH:MM"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpr")
	if err != nil {
		t.Skipf("reference binary gpr not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "default_three_lines",
			Stdin:     []byte("line1\nline2\nline3\n"),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "empty_stdin",
			Stdin:     []byte{},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
		{
			Name:      "multi_page",
			Stdin:     bytes.Repeat([]byte("line\n"), 60),
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{normalizePrDate},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
