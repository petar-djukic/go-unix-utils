// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the moreutils reference binary (not g-prefixed).
const refBinaryName = "sponge"

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// R4: passthrough mode — stdin to stdout.
	t.Run("passthrough_stdout", func(t *testing.T) {
		tests := []testutils.DiffTest{
			{
				Name:  "passthrough_hello",
				Args:  []string{},
				Stdin: []byte("hello world\n"),
			},
			{
				Name:  "passthrough_empty",
				Args:  []string{},
				Stdin: []byte(""),
			},
			{
				Name:  "passthrough_multiline",
				Args:  []string{},
				Stdin: []byte("line1\nline2\nline3\n"),
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1: write stdin to file.
	t.Run("small_stdin_to_file", func(t *testing.T) {
		dir := t.TempDir()
		outFile := filepath.Join(dir, "outfile.txt")
		stdin := generateSeq(1, 100)

		runSpongeAndCompare(t, goBin, refBin, []string{outFile}, stdin, dir)
	})

	// R1: empty stdin to file.
	t.Run("empty_stdin_to_file", func(t *testing.T) {
		dir := t.TempDir()
		outFile := filepath.Join(dir, "empty_out.txt")

		runSpongeAndCompare(t, goBin, refBin, []string{outFile}, []byte(""), dir)
	})

	// R3: soak-before-write contract (cat file | sponge file).
	t.Run("soak_before_write", func(t *testing.T) {
		content := []byte("line1\nline2\nline3\n")

		// Test Go binary.
		goDir := t.TempDir()
		goFile := filepath.Join(goDir, "data.txt")
		if err := os.WriteFile(goFile, content, 0o644); err != nil {
			t.Fatalf("write test file: %v", err)
		}
		runSponge(t, goBin, []string{goFile}, content, goDir)
		goResult, err := os.ReadFile(goFile)
		if err != nil {
			t.Fatalf("read go output: %v", err)
		}

		// Test reference binary.
		refDir := t.TempDir()
		refFile := filepath.Join(refDir, "data.txt")
		if err := os.WriteFile(refFile, content, 0o644); err != nil {
			t.Fatalf("write test file: %v", err)
		}
		runSponge(t, refBin, []string{refFile}, content, refDir)
		refResult, err := os.ReadFile(refFile)
		if err != nil {
			t.Fatalf("read ref output: %v", err)
		}

		if !bytes.Equal(goResult, refResult) {
			t.Errorf("soak-before-write: go=%q ref=%q", goResult, refResult)
		}
		if !bytes.Equal(goResult, content) {
			t.Errorf("soak-before-write: output differs from input: got %q, want %q", goResult, content)
		}
	})

	// R2: append mode.
	t.Run("append_mode", func(t *testing.T) {
		original := []byte("original line\n")
		appended := []byte("appended line\n")

		// Test Go binary.
		goDir := t.TempDir()
		goFile := filepath.Join(goDir, "existing.txt")
		if err := os.WriteFile(goFile, original, 0o644); err != nil {
			t.Fatalf("write test file: %v", err)
		}
		runSponge(t, goBin, []string{"-a", goFile}, appended, goDir)
		goResult, err := os.ReadFile(goFile)
		if err != nil {
			t.Fatalf("read go output: %v", err)
		}

		// Test reference binary.
		refDir := t.TempDir()
		refFile := filepath.Join(refDir, "existing.txt")
		if err := os.WriteFile(refFile, original, 0o644); err != nil {
			t.Fatalf("write test file: %v", err)
		}
		runSponge(t, refBin, []string{"-a", refFile}, appended, refDir)
		refResult, err := os.ReadFile(refFile)
		if err != nil {
			t.Fatalf("read ref output: %v", err)
		}

		if !bytes.Equal(goResult, refResult) {
			t.Errorf("append mode: go=%q ref=%q", goResult, refResult)
		}
	})

	// R2: append mode with non-existent file.
	t.Run("append_mode_no_existing", func(t *testing.T) {
		stdin := []byte("new content\n")

		// Test Go binary.
		goDir := t.TempDir()
		goFile := filepath.Join(goDir, "newfile.txt")
		runSponge(t, goBin, []string{"-a", goFile}, stdin, goDir)
		goResult, err := os.ReadFile(goFile)
		if err != nil {
			t.Fatalf("read go output: %v", err)
		}

		// Test reference binary.
		refDir := t.TempDir()
		refFile := filepath.Join(refDir, "newfile.txt")
		runSponge(t, refBin, []string{"-a", refFile}, stdin, refDir)
		refResult, err := os.ReadFile(refFile)
		if err != nil {
			t.Fatalf("read ref output: %v", err)
		}

		if !bytes.Equal(goResult, refResult) {
			t.Errorf("append no existing: go=%q ref=%q", goResult, refResult)
		}
	})

	// R1: large stdin (>1MB).
	t.Run("large_stdin", func(t *testing.T) {
		stdin := generateSeq(1, 50000)

		goDir := t.TempDir()
		goFile := filepath.Join(goDir, "large_out.txt")
		runSponge(t, goBin, []string{goFile}, stdin, goDir)
		goResult, err := os.ReadFile(goFile)
		if err != nil {
			t.Fatalf("read go output: %v", err)
		}

		refDir := t.TempDir()
		refFile := filepath.Join(refDir, "large_out.txt")
		runSponge(t, refBin, []string{refFile}, stdin, refDir)
		refResult, err := os.ReadFile(refFile)
		if err != nil {
			t.Fatalf("read ref output: %v", err)
		}

		if !bytes.Equal(goResult, refResult) {
			t.Errorf("large stdin: output length go=%d ref=%d", len(goResult), len(refResult))
		}
	})
}

// runSponge executes a sponge binary with the given args, stdin, and working directory.
func runSponge(t *testing.T, binary string, args []string, stdin []byte, workDir string) {
	t.Helper()

	cmd := exec.Command(binary, args...)
	cmd.Dir = workDir
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("%s exited %d: %s", binary, exitErr.ExitCode(), stderr.String())
		}
		t.Fatalf("%s failed: %v", binary, err)
	}
}

// runSpongeAndCompare runs both binaries with the same args and compares output files.
func runSpongeAndCompare(t *testing.T, goBin, refBin string, args []string, stdin []byte, baseDir string) {
	t.Helper()

	// Determine the output file name from args (last non-flag arg).
	var outName string
	for _, a := range args {
		if a[0] != '-' {
			outName = filepath.Base(a)
		}
	}

	// Go binary.
	goDir := t.TempDir()
	goArgs := replaceFilePath(args, baseDir, goDir)
	runSponge(t, goBin, goArgs, stdin, goDir)

	// Reference binary.
	refDir := t.TempDir()
	refArgs := replaceFilePath(args, baseDir, refDir)
	runSponge(t, refBin, refArgs, stdin, refDir)

	// Compare output files.
	goResult, err := os.ReadFile(filepath.Join(goDir, outName))
	if err != nil {
		t.Fatalf("read go output: %v", err)
	}
	refResult, err := os.ReadFile(filepath.Join(refDir, outName))
	if err != nil {
		t.Fatalf("read ref output: %v", err)
	}

	if !bytes.Equal(goResult, refResult) {
		t.Errorf("file content mismatch:\n  go:  %q\n  ref: %q", goResult, refResult)
	}
}

// replaceFilePath replaces oldDir prefix in args with newDir.
func replaceFilePath(args []string, oldDir, newDir string) []string {
	result := make([]string, len(args))
	for i, a := range args {
		if filepath.Dir(a) == oldDir {
			result[i] = filepath.Join(newDir, filepath.Base(a))
		} else {
			result[i] = a
		}
	}
	return result
}

// generateSeq returns output equivalent to `seq 1 N`.
func generateSeq(start, end int) []byte {
	var buf bytes.Buffer
	for i := start; i <= end; i++ {
		fmt.Fprintf(&buf, "%d\n", i)
	}
	return buf.Bytes()
}
