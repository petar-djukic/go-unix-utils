// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildBinary compiles the Go package in dir and returns the path to the
// resulting binary. It calls t.Fatal on build failure.
//
// R2.5: uses go build -o with t.TempDir() for output.
// D1: uses 'go build -o <temppath> <dir>' via os/exec.Command.
// D2: places the compiled binary in t.TempDir() so cleanup is automatic.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()
	binName := resolveBinName(t, dir)
	outPath := filepath.Join(t.TempDir(), binName)

	var stderr bytes.Buffer
	cmd := exec.Command("go", "build", "-o", outPath, dir)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("BuildBinary: go build %s: %v\n%s", dir, err, stderr.String())
	}

	return outPath
}

// resolveBinName determines the binary name from the package directory path.
func resolveBinName(t *testing.T, dir string) string {
	t.Helper()
	binName := filepath.Base(dir)
	if binName == "." {
		abs, err := filepath.Abs(dir)
		if err != nil {
			t.Fatalf("BuildBinary: resolve dir %q: %v", dir, err)
		}
		binName = filepath.Base(abs)
	}
	return binName
}
