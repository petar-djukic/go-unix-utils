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

// BuildBinary compiles the Go package at pkgDir (relative to the caller's
// working directory, typically ".") and returns the path to the compiled
// binary. The binary is placed in t.TempDir() and cleaned up automatically
// via t.Cleanup.
//
// BuildBinary must be called from within the Go module (use "." as pkgDir
// when the test file lives in the cmd/ package being tested). It must not
// create Go files outside the module boundary.
//
// The binary name is derived from the package directory name. When pkgDir
// is ".", the current working directory name is used.
//
// BuildBinary is safe for concurrent use: each call creates its own temp
// directory via t.TempDir(), so parallel tests do not collide on binary paths.
//
// Implements prd001-testutils R4.1-R4.4.
func BuildBinary(t *testing.T, pkgDir string) string {
	t.Helper()

	binName := deriveBinaryName(pkgDir)
	binPath := filepath.Join(t.TempDir(), binName)

	cmd := exec.Command("go", "build", "-o", binPath, pkgDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("BuildBinary: go build %s failed: %v\nstdout:\n%s\nstderr:\n%s",
			pkgDir, err, stdout.Bytes(), stderr.Bytes())
	}

	return binPath
}

// deriveBinaryName returns a binary name from the package directory path.
// When dir is ".", it resolves to the current working directory name.
func deriveBinaryName(dir string) string {
	name := filepath.Base(dir)
	if name == "." {
		wd, err := os.Getwd()
		if err != nil {
			return "testbin"
		}
		name = filepath.Base(wd)
	}
	return name
}
