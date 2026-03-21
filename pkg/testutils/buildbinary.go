// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd001-testutils R5.1-R5.2: BuildBinary test helper.
package testutils

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildBinary compiles the Go package in dir to a temporary binary and
// returns its path. Calls t.Fatal on build failure. The binary is cleaned
// up automatically via t.TempDir. If no .go source files exist in dir
// (e.g., during a generation run where sources are deleted), calls t.Skip
// with a descriptive message.
// Implements prd001-testutils R5.1, R5.2.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()
	skipIfNoGoFiles(t, dir)
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "testbin")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary: go build failed in %s: %v\n%s", dir, err, out)
	}
	return binPath
}

// skipIfNoGoFiles calls t.Skip if dir contains no .go files.
// Implements prd001-testutils R5.2.
func skipIfNoGoFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("BuildBinary: failed to glob .go files in %s: %v", dir, err)
	}
	if len(matches) == 0 {
		t.Skipf("BuildBinary: no .go files in %s, skipping (generation run)", dir)
	}
}
