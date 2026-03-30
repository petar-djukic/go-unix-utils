// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tsort (prd102-tsort R1, R2).
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNameNormalizer replaces the binary name prefix in stderr messages so
// "gtsort:" and "tsort:" (possibly with full path) both become "tsort:".
var stderrNameNormalizer testutils.NormalizeFunc = func() testutils.NormalizeFunc {
	re := regexp.MustCompile(`(?m)^[^\s:]*(?:gtsort|tsort):`)
	return func(data []byte) []byte {
		return re.ReplaceAll(data, []byte("tsort:"))
	}
}()

// stderrTryLineNormalizer strips the "Try ... --help" line GNU appends.
var stderrTryLineNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	re := regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)
	return re.ReplaceAll(data, nil)
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtsort")
	if err != nil {
		t.Skipf("reference binary gtsort not in PATH: %v", err)
	}

	normalizers := []testutils.NormalizeFunc{
		stderrNameNormalizer,
		stderrTryLineNormalizer,
	}

	tests := []testutils.DiffTest{
		// R1.1, R2.1: basic topological sort, exit 0
		{
			Name:      "basic_ordering",
			Stdin:     []byte("a b b c\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R1.1: single pair
		{
			Name:      "single_pair",
			Stdin:     []byte("x y\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R1.1: self-loop pair (a a) — node appears once, no edge
		{
			Name:      "self_pair",
			Stdin:     []byte("a a\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R1.1: multiple independent pairs
		{
			Name:      "independent_pairs",
			Stdin:     []byte("a b\nc d\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R1.1: diamond graph
		{
			Name:      "diamond",
			Stdin:     []byte("a b a c b d c d\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R1.2, R2.2: cycle detection, exit 1
		{
			Name:      "cycle",
			Stdin:     []byte("a b b a\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R1.2: three-node cycle
		{
			Name:      "three_node_cycle",
			Stdin:     []byte("a b b c c a\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R1.3, R2.2: odd number of tokens, exit 1
		{
			Name:      "odd_tokens",
			Stdin:     []byte("a b c\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R1.4: read from stdin with no args
		{
			Name:      "stdin_no_args",
			Stdin:     []byte("p q\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R2.1: empty input, exit 0
		{
			Name:      "empty_input",
			Stdin:     []byte(""),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R2.2: extra operand
		{
			Name:      "extra_operand",
			Args:      []string{"file1", "file2"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R1.1: longer chain
		{
			Name:      "longer_chain",
			Stdin:     []byte("a b b c c d d e\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
		// R1.2: cycle with non-cyclic prefix
		{
			Name:      "partial_cycle",
			Stdin:     []byte("a b b c c b\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		// R1.4: "-" means stdin
		{
			Name:      "dash_means_stdin",
			Args:      []string{"-"},
			Stdin:     []byte("m n\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalizers,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
