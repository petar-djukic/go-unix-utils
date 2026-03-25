// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/users against gusers (GNU coreutils).
// Covers prd096-users R2.2 (exit codes) and R2.3 (error handling).
package main_test

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer replaces the binary name/path prefix in stderr and
// strips the "Try '...' for more information." hint line so that
// structural error messages can be compared across binaries.
func stderrNormalizer(b []byte) []byte {
	// Replace program name prefix: /path/to/gusers: or gusers: or users:
	re := regexp.MustCompile(`(?m)^[^\s:]*(?:gusers|users):`)
	b = re.ReplaceAll(b, []byte("users:"))
	// Strip "Try '...' for more information.\n" lines.
	tryRe := regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)
	b = tryRe.ReplaceAll(b, nil)
	return b
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gusers")
	if err != nil {
		t.Skip("reference binary gusers not in PATH")
	}

	norms := []testutils.NormalizeFunc{stderrNormalizer}

	tests := []testutils.DiffTest{
		// R2.2: default invocation with no arguments exits 0.
		{
			Name: "default_no_args",
			Args: []string{},
		},
		// R2.3: unrecognized option triggers error exit.
		{
			Name:      "unrecognized_option",
			Args:      []string{"--bogus"},
			Normalize: norms,
		},
		// R2.3: extra operand triggers error exit.
		{
			Name:      "extra_operand",
			Args:      []string{"/dev/null", "/dev/null"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
