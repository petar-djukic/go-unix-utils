// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sponge against sponge (moreutils).
// Implements prd007-sponge R1.1-R1.4 test coverage.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skipf("reference binary sponge not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R1.2: passthrough small stdin to stdout (no filename).
		{
			Name:  "R1.2_passthrough_small",
			Stdin: []byte("hello\n"),
		},
		// R1.2: passthrough empty stdin.
		{
			Name:  "R1.2_passthrough_empty",
			Stdin: []byte(""),
		},
		// R1.2: passthrough multi-line.
		{
			Name:  "R1.2_passthrough_multiline",
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		// R1.2: passthrough no trailing newline.
		{
			Name:  "R1.2_passthrough_no_trailing_newline",
			Stdin: []byte("abc"),
		},
		// R1.2: passthrough binary data.
		{
			Name:  "R1.2_passthrough_binary",
			Stdin: []byte{0x00, 0x01, 0xFF, 0xFE, '\n'},
		},
		// R1.4: success exit 0.
		{
			Name:     "R1.4_success_exit_0",
			Stdin:    []byte("data\n"),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestFileOutput verifies that sponge writes stdin content to the named file.
// These are Go-binary-only tests because the differential harness runs both
// binaries in the same WorkDir, and file-output tests need isolated state.
func TestFileOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// R1.1, R1.3: write stdin to a new file.
	t.Run("R1.3_write_new_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "out.txt")
		input := []byte("hello\nworld\n")

		runSponge(t, goBin, dir, input, outFile)

		got := readFile(t, outFile)
		if !bytes.Equal(got, input) {
			t.Errorf("expected %q, got %q", input, got)
		}
	})

	// R1.3: write stdin to an existing file (truncates).
	t.Run("R1.3_truncate_existing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "existing.txt")
		writeFile(t, outFile, "old content that is longer\n")
		input := []byte("new\n")

		runSponge(t, goBin, dir, input, outFile)

		got := readFile(t, outFile)
		if !bytes.Equal(got, input) {
			t.Errorf("expected %q, got %q", input, got)
		}
	})

	// R1.1: soak-before-write contract — reading from and writing to the
	// same file must not produce empty output.
	t.Run("R1.1_soak_same_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "data.txt")
		original := []byte("original content\n")
		writeFile(t, outFile, string(original))

		// cat data.txt | sponge data.txt — must produce original content.
		catBin, err := exec.LookPath("cat")
		if err != nil {
			t.Skip("cat not found in PATH")
		}

		bashBin, err := exec.LookPath("bash")
		if err != nil {
			t.Skip("bash not found in PATH")
		}

		cmd := exec.Command(bashBin, "-c",
			catBin+" "+outFile+" | "+goBin+" "+outFile)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("soak-before-write test failed: %v\noutput: %s", err, out)
		}

		got := readFile(t, outFile)
		if !bytes.Equal(got, original) {
			t.Errorf("soak-before-write violated: expected %q, got %q", original, got)
		}
	})

	// R1.3: write empty stdin to file.
	t.Run("R1.3_empty_stdin", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "empty.txt")

		runSponge(t, goBin, dir, []byte{}, outFile)

		got := readFile(t, outFile)
		if len(got) != 0 {
			t.Errorf("expected empty file, got %q", got)
		}
	})

	// R1.4: exit non-zero on unwritable path.
	t.Run("R1.4_unwritable_path", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Create a read-only directory so the file cannot be created.
		roDir := filepath.Join(dir, "readonly")
		if err := os.Mkdir(roDir, 0o555); err != nil {
			t.Fatalf("creating read-only dir: %v", err)
		}
		outFile := filepath.Join(roDir, "out.txt")

		cmd := exec.Command(goBin, outFile)
		cmd.Stdin = bytes.NewReader([]byte("data\n"))
		cmd.Dir = dir
		err := cmd.Run()
		if err == nil {
			t.Error("expected non-zero exit for unwritable path, got exit 0")
		}
	})
}

// runSponge runs the sponge binary with the given stdin and output file argument.
func runSponge(t *testing.T, bin, dir string, stdin []byte, outFile string) {
	t.Helper()
	cmd := exec.Command(bin, outFile)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sponge failed: %v\noutput: %s", err, out)
	}
}

// writeFile creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing file %s: %v", path, err)
	}
}

// readFile reads and returns the contents of a file.
func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file %s: %v", path, err)
	}
	return data
}
