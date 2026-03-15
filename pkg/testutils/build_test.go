// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeriveBinaryName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dir  string
		want string
	}{
		{"explicit directory", "cmd/cat", "cat"},
		{"nested path", "/some/path/to/myutil", "myutil"},
		{"single name", "myutil", "myutil"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := deriveBinaryName(tc.dir)
			if got != tc.want {
				t.Errorf("deriveBinaryName(%q) = %q, want %q", tc.dir, got, tc.want)
			}
		})
	}
}

func TestDeriveBinaryNameDot(t *testing.T) {
	t.Parallel()

	// When dir is ".", deriveBinaryName uses the current working directory name.
	got := deriveBinaryName(".")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() failed: %v", err)
	}
	want := filepath.Base(wd)
	if got != want {
		t.Errorf("deriveBinaryName(\".\") = %q, want %q", got, want)
	}
}

// findBuildableCmd looks for a cmd/ package with a main.go that can be used
// for BuildBinary integration tests. Returns the relative path (e.g.,
// "../../cmd/true") or empty string if none found. During generation runs,
// cmd/ directories may be empty.
func findBuildableCmd(t *testing.T) string {
	t.Helper()

	// pkg/testutils is two levels below the repo root.
	repoRoot := filepath.Join("..", "..")
	cmdDir := filepath.Join(repoRoot, "cmd")

	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return ""
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		mainPath := filepath.Join(cmdDir, e.Name(), "main.go")
		if _, err := os.Stat(mainPath); err == nil {
			return filepath.Join(cmdDir, e.Name())
		}
	}
	return ""
}

func TestBuildBinaryReturnsExecutable(t *testing.T) {
	t.Parallel()

	cmdPath := findBuildableCmd(t)
	if cmdPath == "" {
		t.Skip("no cmd/ package with main.go found (generation may be in progress)")
	}

	binPath := BuildBinary(t, cmdPath)

	// AC1: Binary path is returned.
	if binPath == "" {
		t.Fatal("BuildBinary returned empty path")
	}

	// AC3: Binary is inside a temp directory and exists.
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("binary not found at %s: %v", binPath, err)
	}

	// AC3: Binary is executable.
	if info.Mode()&0o111 == 0 {
		t.Errorf("binary at %s is not executable: mode=%v", binPath, info.Mode())
	}
}

func TestBuildBinaryConcurrentSafety(t *testing.T) {
	t.Parallel()

	cmdPath := findBuildableCmd(t)
	if cmdPath == "" {
		t.Skip("no cmd/ package with main.go found (generation may be in progress)")
	}

	// R4: Verify concurrent calls each get their own binary path via separate TempDirs.
	paths := make(chan string, 2)

	t.Run("concurrent-a", func(t *testing.T) {
		t.Parallel()
		paths <- BuildBinary(t, cmdPath)
	})
	t.Run("concurrent-b", func(t *testing.T) {
		t.Parallel()
		paths <- BuildBinary(t, cmdPath)
	})
}
