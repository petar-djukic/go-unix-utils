// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// goBin is the name of the Go compiler binary.
const goBin = "go"

// BuildBinary compiles the cmd/ package at dir and returns the path to the
// built binary. Calls t.Skip if no Go source files exist (generation in
// progress). Calls t.Fatal with go build stderr on build failure.
// R4.1: compile a cmd/ package into a temporary binary for testing.
// R4.2: uses t.TempDir() for automatic cleanup.
// R4.3: skips gracefully when source is missing.
// R4.4: reports go build stderr on failure.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()
	checkSourceExists(t, dir)
	binPath := filepath.Join(t.TempDir(), "binary")
	buildPackage(t, dir, binPath)
	return binPath
}

// checkSourceExists verifies that main.go exists in the package directory.
// R4.3: skip the test instead of failing when source is absent.
func checkSourceExists(t *testing.T, dir string) {
	t.Helper()
	mainPath := filepath.Join(dir, "main.go")
	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		t.Skipf("source not found: %s (generation in progress)", mainPath)
	}
}

// buildPackage runs go build and captures stderr for error reporting.
// R4.4: includes compilation error message in test output on failure.
func buildPackage(t *testing.T, dir, binPath string) {
	t.Helper()
	cmd := exec.Command(goBin, "build", "-o", binPath, dir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build %s failed: %v\n%s", dir, err, stderr.String())
	}
}
