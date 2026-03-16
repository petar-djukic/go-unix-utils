// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tee against gtee (GNU coreutils).
// Implements prd017-tee R1.1-R1.5, R2.1-R2.3, R3.1-R3.4, R4.1-R4.3 test coverage.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing stdout output and exit codes
// between the Go tee binary and the GNU gtee reference binary.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R1.2: passthrough mode — no file arguments.
		{
			Name:  "R1.2_passthrough_hello",
			Stdin: []byte("hello\n"),
		},
		// R1.2: passthrough empty stdin.
		{
			Name:  "R1.2_passthrough_empty",
			Stdin: []byte(""),
		},
		// R1.1: passthrough multi-line.
		{
			Name:  "R1.1_passthrough_multiline",
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		// R1.1: passthrough no trailing newline.
		{
			Name:  "R1.1_passthrough_no_trailing_newline",
			Stdin: []byte("abc"),
		},
		// R1.1: passthrough binary data.
		{
			Name:  "R1.1_passthrough_binary",
			Stdin: []byte{0x00, 0x01, 0xFF, 0xFE, '\n'},
		},
		// R1.4: "-" as file argument — stdout only, no duplication.
		{
			Name:  "R1.4_dash_as_file",
			Args:  []string{"-"},
			Stdin: []byte("dash test\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffAppend runs differential tests for -a (append) mode.
func TestDiffAppend(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}

	input := []byte("appended line\n")

	for _, label := range []string{"go", "ref"} {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			outFile := filepath.Join(dir, "out.txt")

			// Pre-populate the file with existing content.
			writeFile(t, outFile, "existing content\n")

			bin := goBin
			if label == "ref" {
				bin = refBin
			}

			cmd := exec.Command(bin, "-a", outFile)
			cmd.Stdin = bytes.NewReader(input)
			cmd.Dir = dir
			stdout, runErr := cmd.Output()
			if runErr != nil {
				t.Fatalf("tee -a failed: %v", runErr)
			}

			// R2.1: stdout must match stdin.
			if !bytes.Equal(stdout, input) {
				t.Errorf("stdout: expected %q, got %q", input, stdout)
			}

			// R2.1: file must contain existing content + appended data.
			want := []byte("existing content\nappended line\n")
			got := readFile(t, outFile)
			if !bytes.Equal(got, want) {
				t.Errorf("file: expected %q, got %q", want, got)
			}
		})
	}
}

// TestDiffIgnoreInterrupts runs differential tests for -i (ignore-interrupts).
func TestDiffIgnoreInterrupts(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}

	// R2.2: -i flag — tee should still pass through data normally.
	tests := []testutils.DiffTest{
		{
			Name:  "R2.2_ignore_interrupts_passthrough",
			Args:  []string{"-i"},
			Stdin: []byte("sigint test\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCombinedFlags runs differential tests with -a and -i combined.
func TestDiffCombinedFlags(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}

	// R2.3: -a and -i combined — stdout should match.
	tests := []testutils.DiffTest{
		{
			Name:  "R2.3_append_and_ignore_combined",
			Args:  []string{"-a", "-i"},
			Stdin: []byte("combined flags\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffWriteError runs differential tests for write-error handling.
func TestDiffWriteError(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}

	// Set up a read-only directory for permission-denied tests.
	permDir := t.TempDir()
	roDir := filepath.Join(permDir, "readonly")
	if err := os.Mkdir(roDir, 0o555); err != nil {
		t.Fatalf("creating read-only dir: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.2: permission denied on output file — both must exit 1.
		// Stdout still receives the data.
		{
			Name:      "R3.2_permission_denied",
			Args:      []string{filepath.Join(roDir, "out.txt")},
			Stdin:     []byte("data\n"),
			ExitCode:  1,
			WorkDir:   permDir,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestFileOutput verifies that tee writes stdin content to named output files.
// These are Go-binary-only tests for file content verification.
func TestFileOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// R1.1, R1.3: write stdin to a single file.
	t.Run("R1.1_single_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "out.txt")
		input := []byte("hello\nworld\n")

		stdout := runTee(t, goBin, dir, input, outFile)

		// Stdout must match stdin.
		if !bytes.Equal(stdout, input) {
			t.Errorf("stdout: expected %q, got %q", input, stdout)
		}
		// File must match stdin.
		got := readFile(t, outFile)
		if !bytes.Equal(got, input) {
			t.Errorf("file: expected %q, got %q", input, got)
		}
	})

	// R1.1: write stdin to multiple files.
	t.Run("R1.1_multiple_files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		f1 := filepath.Join(dir, "f1.txt")
		f2 := filepath.Join(dir, "f2.txt")
		input := []byte("data for multiple files\n")

		stdout := runTee(t, goBin, dir, input, f1, f2)

		if !bytes.Equal(stdout, input) {
			t.Errorf("stdout: expected %q, got %q", input, stdout)
		}
		for _, path := range []string{f1, f2} {
			got := readFile(t, path)
			if !bytes.Equal(got, input) {
				t.Errorf("file %s: expected %q, got %q", path, input, got)
			}
		}
	})

	// R1.3: existing file is truncated.
	t.Run("R1.3_truncate_existing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "existing.txt")
		writeFile(t, outFile, "old content that is longer\n")
		input := []byte("new\n")

		runTee(t, goBin, dir, input, outFile)

		got := readFile(t, outFile)
		if !bytes.Equal(got, input) {
			t.Errorf("expected %q, got %q", input, got)
		}
	})

	// R1.2: passthrough only — no file arguments.
	t.Run("R1.2_passthrough_only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		input := []byte("passthrough\n")

		cmd := exec.Command(goBin)
		cmd.Stdin = bytes.NewReader(input)
		cmd.Dir = dir
		stdout, err := cmd.Output()
		if err != nil {
			t.Fatalf("tee passthrough failed: %v", err)
		}
		if !bytes.Equal(stdout, input) {
			t.Errorf("expected %q, got %q", input, stdout)
		}
	})

	// R1.1: empty stdin produces empty file.
	t.Run("R1.1_empty_stdin", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "empty.txt")

		runTee(t, goBin, dir, []byte{}, outFile)

		got := readFile(t, outFile)
		if len(got) != 0 {
			t.Errorf("expected empty file, got %q", got)
		}
	})

	// R3.3: one failed file does not prevent writing to others.
	t.Run("R3.3_continue_on_error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		roDir := filepath.Join(dir, "readonly")
		if err := os.Mkdir(roDir, 0o555); err != nil {
			t.Fatalf("creating read-only dir: %v", err)
		}
		badFile := filepath.Join(roDir, "bad.txt")
		goodFile := filepath.Join(dir, "good.txt")
		input := []byte("data\n")

		cmd := exec.Command(goBin, badFile, goodFile)
		cmd.Stdin = bytes.NewReader(input)
		cmd.Dir = dir
		var stdoutBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		err := cmd.Run()

		// Should exit non-zero due to bad file.
		if err == nil {
			t.Error("expected non-zero exit for failed file, got exit 0")
		}

		// Good file should still have the data.
		got := readFile(t, goodFile)
		if !bytes.Equal(got, input) {
			t.Errorf("good file: expected %q, got %q", input, got)
		}

		// Stdout should still have the data.
		if !bytes.Equal(stdoutBuf.Bytes(), input) {
			t.Errorf("stdout: expected %q, got %q", input, stdoutBuf.Bytes())
		}
	})

	// R2.1: append mode preserves existing content.
	t.Run("R2.1_append_mode", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "append.txt")
		writeFile(t, outFile, "old\n")
		input := []byte("new\n")

		cmd := exec.Command(goBin, "-a", outFile)
		cmd.Stdin = bytes.NewReader(input)
		cmd.Dir = dir
		stdout, err := cmd.Output()
		if err != nil {
			t.Fatalf("tee -a failed: %v", err)
		}

		// Stdout gets only the new data.
		if !bytes.Equal(stdout, input) {
			t.Errorf("stdout: expected %q, got %q", input, stdout)
		}

		// File has old + new.
		want := []byte("old\nnew\n")
		got := readFile(t, outFile)
		if !bytes.Equal(got, want) {
			t.Errorf("file: expected %q, got %q", want, got)
		}
	})

	// R2.1: append to multiple files.
	t.Run("R2.1_append_multiple_files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		f1 := filepath.Join(dir, "f1.txt")
		f2 := filepath.Join(dir, "f2.txt")
		writeFile(t, f1, "a\n")
		writeFile(t, f2, "b\n")
		input := []byte("c\n")

		cmd := exec.Command(goBin, "-a", f1, f2)
		cmd.Stdin = bytes.NewReader(input)
		cmd.Dir = dir
		if _, err := cmd.Output(); err != nil {
			t.Fatalf("tee -a failed: %v", err)
		}

		got1 := readFile(t, f1)
		if !bytes.Equal(got1, []byte("a\nc\n")) {
			t.Errorf("f1: expected %q, got %q", "a\nc\n", got1)
		}
		got2 := readFile(t, f2)
		if !bytes.Equal(got2, []byte("b\nc\n")) {
			t.Errorf("f2: expected %q, got %q", "b\nc\n", got2)
		}
	})

	// R2.2: -i flag still copies data correctly.
	t.Run("R2.2_ignore_interrupts_file_output", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "sigint.txt")
		input := []byte("data with -i\n")

		cmd := exec.Command(goBin, "-i", outFile)
		cmd.Stdin = bytes.NewReader(input)
		cmd.Dir = dir
		stdout, err := cmd.Output()
		if err != nil {
			t.Fatalf("tee -i failed: %v", err)
		}

		if !bytes.Equal(stdout, input) {
			t.Errorf("stdout: expected %q, got %q", input, stdout)
		}
		got := readFile(t, outFile)
		if !bytes.Equal(got, input) {
			t.Errorf("file: expected %q, got %q", input, got)
		}
	})

	// R3.2: error message on stderr when file cannot be opened.
	t.Run("R3.2_error_message_on_stderr", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		roDir := filepath.Join(dir, "readonly")
		if err := os.Mkdir(roDir, 0o555); err != nil {
			t.Fatalf("creating read-only dir: %v", err)
		}
		badFile := filepath.Join(roDir, "bad.txt")
		input := []byte("data\n")

		cmd := exec.Command(goBin, badFile)
		cmd.Stdin = bytes.NewReader(input)
		cmd.Dir = dir
		var stderrBuf bytes.Buffer
		cmd.Stderr = &stderrBuf
		_ = cmd.Run() // expect exit 1

		stderr := stderrBuf.String()
		if !bytes.Contains([]byte(stderr), []byte("tee:")) {
			t.Errorf("expected stderr to contain 'tee:', got %q", stderr)
		}
		if !bytes.Contains([]byte(stderr), []byte(badFile)) {
			t.Errorf("expected stderr to contain file path %q, got %q", badFile, stderr)
		}
	})

	// R4.3: file content matches stdout byte-for-byte.
	t.Run("R4.3_file_matches_stdout", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "verify.txt")
		input := []byte("verify byte-for-byte\x00\x01\xFF\n")

		stdout := runTee(t, goBin, dir, input, outFile)

		got := readFile(t, outFile)
		if !bytes.Equal(got, stdout) {
			t.Errorf("file content does not match stdout\nfile:   %q\nstdout: %q", got, stdout)
		}
	})
}

// TestFileOutputDiff runs differential tests comparing file output between
// the Go binary and the GNU gtee reference binary.
func TestFileOutputDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}

	input := []byte("hello\nworld\n")

	for _, label := range []string{"go", "ref"} {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			outFile := filepath.Join(dir, "out.txt")

			bin := goBin
			if label == "ref" {
				bin = refBin
			}

			cmd := exec.Command(bin, outFile)
			cmd.Stdin = bytes.NewReader(input)
			cmd.Dir = dir
			stdout, runErr := cmd.Output()
			if runErr != nil {
				t.Fatalf("tee failed: %v", runErr)
			}

			// Stdout must match stdin.
			if !bytes.Equal(stdout, input) {
				t.Errorf("stdout: expected %q, got %q", input, stdout)
			}

			// File must match stdin.
			got := readFile(t, outFile)
			if !bytes.Equal(got, input) {
				t.Errorf("file: expected %q, got %q", input, got)
			}
		})
	}
}

// TestMultipleFilesDiff runs differential tests for multiple file output.
func TestMultipleFilesDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}

	input := []byte("multi-file data\n")

	for _, label := range []string{"go", "ref"} {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			f1 := filepath.Join(dir, "f1.txt")
			f2 := filepath.Join(dir, "f2.txt")

			bin := goBin
			if label == "ref" {
				bin = refBin
			}

			cmd := exec.Command(bin, f1, f2)
			cmd.Stdin = bytes.NewReader(input)
			cmd.Dir = dir
			stdout, runErr := cmd.Output()
			if runErr != nil {
				t.Fatalf("tee failed: %v", runErr)
			}

			if !bytes.Equal(stdout, input) {
				t.Errorf("stdout: expected %q, got %q", input, stdout)
			}
			for _, path := range []string{f1, f2} {
				got := readFile(t, path)
				if !bytes.Equal(got, input) {
					t.Errorf("file %s: expected %q, got %q", filepath.Base(path), input, got)
				}
			}
		})
	}
}

// runTee runs the tee binary with the given stdin and file arguments,
// returning stdout. Fails the test on non-zero exit.
func runTee(t *testing.T, bin, dir string, stdin []byte, files ...string) []byte {
	t.Helper()
	cmd := exec.Command(bin, files...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Dir = dir
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("tee failed: %v", err)
	}
	return stdout
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

// clearStderr returns empty bytes, allowing differential comparison to focus
// on exit code and stdout only. Used for error condition tests where the
// error message format differs between Go and GNU implementations.
func clearStderr(b []byte) []byte {
	return []byte{}
}
