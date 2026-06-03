// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestVersionDefault(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	out, err := exec.Command(bin).Output()
	if err != nil {
		t.Fatalf("version binary failed: %v", err)
	}
	got := strings.TrimRight(string(out), "\n")
	if got != "dev" {
		t.Fatalf("expected \"dev\", got %q", got)
	}
}

func TestVersionFlag(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	for _, flag := range []string{"--version", "-v"} {
		t.Run(flag, func(t *testing.T) {
			out, err := exec.Command(bin, flag).Output()
			if err != nil {
				t.Fatalf("version %s failed: %v", flag, err)
			}
			got := strings.TrimRight(string(out), "\n")
			if got != "dev" {
				t.Fatalf("expected \"dev\", got %q", got)
			}
		})
	}
}

func TestVersionUnknownFlag(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(bin, "--bogus")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for unknown flag")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.ExitCode())
	}
}

// R1.5: verify the Version variable is exported and has the expected default.
func TestVersionExported(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("expected exported Version to be \"dev\", got %q", Version)
	}
}
