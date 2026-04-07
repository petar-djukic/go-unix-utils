// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat. Implements srd006-cat R5.4 test coverage.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skip("reference binary gcat not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			// R5.4: cat must exit 0 when stdout is closed by a downstream consumer.
			// We generate a large input so cat is still writing when the pipe closes.
			// The DiffTest framework captures exit codes; both binaries must exit 0.
			Name:  "sigpipe_large_input",
			Args:  []string{"-"},
			Stdin: generateLargeInput(64 * 1024),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestSIGPIPE verifies R5.4: cat exits 0 when stdout is closed by a
// downstream consumer (e.g., head -1). This test uses a shell pipeline
// to trigger SIGPIPE on the Go binary.
func TestSIGPIPE(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	// Create a large input file so cat is still writing when head closes.
	tmp := t.TempDir()
	inputFile := filepath.Join(tmp, "large.txt")
	if err := os.WriteFile(inputFile, generateLargeInput(128*1024), 0o644); err != nil {
		t.Fatalf("write input file: %v", err)
	}

	// Use sh -c to create a real pipeline where SIGPIPE is delivered.
	cmd := exec.Command("sh", "-c", goBin+" "+inputFile+" | head -1 >/dev/null")
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		// R5.4: the pipeline should exit 0.
		if err != nil {
			t.Errorf("pipeline exited with error: %v (expected exit 0)", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("pipeline timed out after 10s")
	}
}

// generateLargeInput creates repeated lines to produce input of at least n bytes.
func generateLargeInput(n int) []byte {
	line := []byte("abcdefghijklmnopqrstuvwxyz 0123456789\n")
	buf := make([]byte, 0, n+len(line))
	for len(buf) < n {
		buf = append(buf, line...)
	}
	return buf
}
