// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd001-testutils R4.1
package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildBinary compiles the Go package at dir and returns the path to the
// resulting executable binary. The binary is placed in t.TempDir() and
// removed automatically when the test completes.
//
// R4.1: BuildBinary compiles the package at dir, places the binary in
// t.TempDir(), and returns its path. Cleanup is handled by t.TempDir().
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "testbin")

	// Run go build with Dir set to the package directory so the build occurs
	// in the correct module context, regardless of the test's working directory.
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = dir
	// Capture combined output for the failure message; do not write to test stdout/stderr during a passing build.
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary: failed to compile %q: %v\n%s", dir, err, out)
	}

	// Verify the binary was produced and is executable.
	info, statErr := os.Stat(binPath)
	if statErr != nil {
		t.Fatalf("BuildBinary: binary not found at %q after build: %v", binPath, statErr)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("BuildBinary: binary %q is not executable (mode %v)", binPath, info.Mode())
	}

	return binPath
}
