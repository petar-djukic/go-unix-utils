// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var progNameRe = regexp.MustCompile(`(?:/\S+/)?g?vdir\b`)

func programNameNormalizer(b []byte) []byte {
	return progNameRe.ReplaceAll(b, []byte("vdir"))
}

func touchFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gvdir")
	if err != nil {
		t.Skip("reference binary not found")
	}

	errNorm := []testutils.NormalizeFunc{programNameNormalizer}

	sigpipeDir := t.TempDir()
	for i := range 200 {
		touchFile(t, sigpipeDir, fmt.Sprintf("file_%03d", i))
	}

	tests := []testutils.DiffTest{
		// R2.3: exit 2 on invalid short option
		{
			Name:      "invalid-short-option",
			Args:      []string{"-j"},
			ExitCode:  2,
			Normalize: errNorm,
		},
		// R2.3: exit 2 on invalid long option
		{
			Name:      "invalid-long-option",
			Args:      []string{"--bogus-flag"},
			ExitCode:  2,
			Normalize: errNorm,
		},
		// R2.4: large output exercises SIGPIPE handling when piped
		{
			Name: "sigpipe-large-output",
			Args: []string{sigpipeDir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
