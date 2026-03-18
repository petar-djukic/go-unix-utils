// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd059-version R1.1–R1.5: version command binary.
package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestVersionNoArgs verifies that invoking the binary with no arguments
// prints "dev" followed by a newline and exits 0. Covers R1.1, R1.2.
func TestVersionNoArgs(t *testing.T) {
	t.Parallel()
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v", err)
	}

	got := string(out)
	if got != "dev\n" {
		t.Fatalf("expected %q, got %q", "dev\n", got)
	}
}

// TestVersionFlag verifies that --version prints the same output as
// no-argument invocation. Covers R1.4.
func TestVersionFlag(t *testing.T) {
	t.Parallel()
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v", err)
	}

	got := string(out)
	if got != "dev\n" {
		t.Fatalf("expected %q, got %q", "dev\n", got)
	}
}

// TestVersionShortFlag verifies that -v prints the same output as
// no-argument invocation. Covers R1.4.
func TestVersionShortFlag(t *testing.T) {
	t.Parallel()
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-v")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v", err)
	}

	got := string(out)
	if got != "dev\n" {
		t.Fatalf("expected %q, got %q", "dev\n", got)
	}
}

// TestVersionUnknownFlag verifies that an unknown flag prints a usage
// message to stderr and exits 2. Covers R1.4.
func TestVersionUnknownFlag(t *testing.T) {
	t.Parallel()
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "--bogus")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit, got exit 0")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.ExitCode())
	}

	combined := string(out)
	if !strings.Contains(combined, "usage:") {
		t.Fatalf("expected usage message in output, got %q", combined)
	}
}

// TestVersionExportedVar verifies that the version variable is accessible
// within the package (exported to other packages via the exported pattern).
// Covers R1.5. Since we are in the same package, we verify directly.
func TestVersionExportedVar(t *testing.T) {
	t.Parallel()
	if version == "" {
		t.Fatal("version variable must not be empty")
	}
	if version != "dev" {
		t.Fatalf("expected default version %q, got %q", "dev", version)
	}
}
