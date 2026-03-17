// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// BuildBinary compiles a cmd/ package and returns the path to the built binary.
// R2.1: uses go build within the module boundary per shared_protocols.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "binary")
	cmd := exec.Command("go", "build", "-o", binPath, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary: go build %s failed: %v\n%s", dir, err, out)
	}
	return binPath
}
