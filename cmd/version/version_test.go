// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// buildVersionBinary compiles cmd/version without ldflags, producing a dev build.
func buildVersionBinary(t *testing.T) string {
	t.Helper()
	return testutils.BuildBinary(t, ".")
}

func TestVersion_NoArgs_PrintsDev(t *testing.T) {
	t.Parallel()
	bin := buildVersionBinary(t)

	// AC4: development build (no ldflags) prints "dev".
	cmd := exec.Command(bin)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running version binary: %v", err)
	}

	got := string(out)
	if got != "dev\n" {
		t.Errorf("version output = %q, want %q", got, "dev\n")
	}
}

func TestVersion_VersionFlag(t *testing.T) {
	t.Parallel()
	bin := buildVersionBinary(t)

	// AC2: --version prints the same output as no arguments.
	for _, flag := range []string{"--version", "-v"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(bin, flag)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("running version %s: %v", flag, err)
			}
			got := string(out)
			if got != "dev\n" {
				t.Errorf("version %s output = %q, want %q", flag, got, "dev\n")
			}
		})
	}
}

func TestVersion_UnknownFlag_ExitsTwo(t *testing.T) {
	t.Parallel()
	bin := buildVersionBinary(t)

	// AC3: unknown flag prints usage to stderr and exits 2.
	cmd := exec.Command(bin, "--bogus")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for --bogus, got nil error")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2", exitErr.ExitCode())
	}

	stderrStr := stderr.String()
	if !strings.Contains(strings.ToLower(stderrStr), "usage") {
		t.Errorf("stderr = %q, want it to contain \"usage\"", stderrStr)
	}
}

func TestVersion_ExitCodeZero(t *testing.T) {
	t.Parallel()
	bin := buildVersionBinary(t)

	// AC2: exit code is 0 for no-arg invocation.
	cmd := exec.Command(bin)
	if err := cmd.Run(); err != nil {
		t.Errorf("version binary exited with error: %v", err)
	}
}

func TestVersion_MultipleArgs_ExitsTwo(t *testing.T) {
	t.Parallel()
	bin := buildVersionBinary(t)

	// R1.4: multiple arguments should also trigger usage/exit 2.
	cmd := exec.Command(bin, "--version", "extra")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for multiple args, got nil error")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2", exitErr.ExitCode())
	}
}
