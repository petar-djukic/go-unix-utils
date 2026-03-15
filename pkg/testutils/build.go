// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd001-testutils R4.1-R4.4: BuildBinary test helper for compiling
// Go binaries during differential testing.

package testutils

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildBinary compiles the Go package at pkgPath and returns the path to the
// resulting binary. The binary is placed in a temporary directory managed by
// t.TempDir(), so it is automatically cleaned up when the test finishes.
//
// pkgPath is a Go package path suitable for go build (e.g., "." for the
// current package, or "./cmd/cat" for a sub-package). The binary name is
// derived from the last element of the resolved package path.
//
// R4.1: per-test binary compilation for differential testing.
// R4.2: uses t.Fatal on compilation failure so the test stops immediately.
func BuildBinary(t *testing.T, pkgPath string) string {
	t.Helper()

	// Resolve the absolute path of the package directory so the binary name
	// can be derived from the directory name.
	absPkg, err := filepath.Abs(pkgPath)
	if err != nil {
		t.Fatalf("BuildBinary: resolving package path %q: %v", pkgPath, err)
	}

	binName := filepath.Base(absPkg)
	binPath := filepath.Join(t.TempDir(), binName)

	// R4.3: compile with go build, capturing stderr for error reporting.
	// R4.4: the binary is placed in a t.TempDir() for automatic cleanup.
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = absPkg
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary: go build %s failed:\n%s\n%v", pkgPath, output, err)
	}

	return binPath
}
