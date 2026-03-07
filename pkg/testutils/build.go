// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildBinary compiles the Go package at pkgPath to a temporary binary and
// returns the path to the compiled binary. The binary is removed via t.Cleanup
// when the test completes. This is the standard way cmd/ tests obtain their Go
// binary for differential testing.
func BuildBinary(t *testing.T, pkgPath string) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "testbin")
	cmd := exec.Command("go", "build", "-o", binPath, pkgPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("building %s: %v\n%s", pkgPath, err, out)
	}
	return binPath
}
