// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tsort against gtsort.
// Implements prd102-tsort R2.1, R2.2, R2.3.
package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer replaces the binary name in stderr so that
// "gtsort:" and "tsort:" compare as equal.
func stderrNormalizer() testutils.NormalizeFunc {
	return func(b []byte) []byte {
		return bytes.ReplaceAll(b, []byte("gtsort"), []byte("tsort"))
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtsort")
	if err != nil {
		t.Skip("reference binary gtsort not in PATH")
	}

	errNorm := stderrNormalizer()

	tests := []testutils.DiffTest{
		// R2.1: simple linear chains — exit 0, deterministic order.
		{
			Name:  "linear_chain_three",
			Stdin: []byte("a b b c\n"),
		},
		{
			Name:  "linear_chain_five",
			Stdin: []byte("a b b c c d d e\n"),
		},
		// R2.1: diamond DAG — both orderings are valid, so we
		// use a fully-constrained diamond with a unique order.
		{
			Name:  "diamond_constrained",
			Stdin: []byte("a b a c b d c d b c\n"),
		},
		// R2.1: disconnected components — exit 0.
		{
			Name:  "disconnected_components",
			Stdin: []byte("a b c d\n"),
		},
		{
			Name:  "disconnected_three_components",
			Stdin: []byte("a b c d e f\n"),
		},
		// R2.1: single edge — exit 0.
		{
			Name:  "single_edge",
			Stdin: []byte("x y\n"),
		},
		// R2.3: single node (self-pair) — exit 0.
		{
			Name:  "single_node_self_pair",
			Stdin: []byte("a a\n"),
		},
		// R2.3: empty input — exit 0.
		{
			Name:  "empty_input",
			Stdin: []byte(""),
		},
		{
			Name:  "whitespace_only",
			Stdin: []byte("   \n\n  \n"),
		},
		// R2.3: duplicate edges — exit 0.
		{
			Name:  "duplicate_edges",
			Stdin: []byte("a b a b a b\n"),
		},
		// R2.2: single cycle — exit 1, stderr matches.
		{
			Name:      "single_cycle",
			Stdin:     []byte("a b b c c a\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.2: self-loop — exit 1.
		{
			Name:      "self_loop",
			Stdin:     []byte("a b b b\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.2: multiple cycles — exit 1.
		{
			Name:      "multiple_cycles",
			Stdin:     []byte("a b b a c d d c\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.2: cycle with non-cycle nodes — exit 1.
		{
			Name:      "cycle_with_chain",
			Stdin:     []byte("x y a b b c c a\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.3: odd token count — exit 1.
		{
			Name:      "odd_token_count",
			Stdin:     []byte("a b c\n"),
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.3: large graph — exit 0.
		{
			Name:  "large_linear_chain",
			Stdin: []byte(buildLargeChain(200)),
		},
		// R2.3: multiline input — exit 0.
		{
			Name:  "multiline_input",
			Stdin: []byte("a b\nb c\nc d\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildLargeChain generates a linear chain of n nodes as tsort input.
func buildLargeChain(n int) string {
	var b strings.Builder
	for i := 0; i < n-1; i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "n%d n%d", i, i+1)
	}
	b.WriteByte('\n')
	return b.String()
}
