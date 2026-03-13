// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tee.
//
// Implements: prd017-tee R3.1–R3.4, R4.1, R4.2, R4.3
package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const binGtee = "gtee"

// teeErrRe matches a tee or gtee error line and normalizes the program name
// and error message format differences between the Go and GNU implementations.
var teeErrRe = regexp.MustCompile(`(?m)^g?tee: (?:open )?(.+?): .+$`)

// normalizeTeeErrors replaces tee/gtee error lines with a canonical form.
func normalizeTeeErrors(b []byte) []byte {
	return teeErrRe.ReplaceAll(b, []byte("PROG: $1: ERROR"))
}

// TestDiff runs differential tests comparing stdout, stderr, and exit codes
// between the Go tee binary and the GNU gtee reference binary.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGtee)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGtee, err)
	}

	dir := t.TempDir()
	readOnlyPath := filepath.Join(dir, "no-such-dir", "file.txt")

	tests := []testutils.DiffTest{
		// R1.2: passthrough mode — no file arguments, stdin copied to stdout only.
		{
			Name:     "r1.2_passthrough_stdin_to_stdout",
			Stdin:    []byte("hello\nworld\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: empty stdin — exits 0, no output.
		{
			Name:     "r1.1_empty_stdin",
			Stdin:    []byte{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: binary data passes through unchanged.
		{
			Name:     "r1.1_binary_data",
			Stdin:    []byte("\x00\x01\x02\xff\xfe\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.2: write to a nonexistent directory fails with exit 1.
		{
			Name:      "r3.2_write_error_exit1",
			Args:      []string{readOnlyPath},
			Stdin:     []byte("data\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeTeeErrors},
		},
		// R1.5: output order preserved — passthrough with data.
		{
			Name:     "r1.5_output_order_preserved",
			Stdin:    []byte("first\nsecond\nthird\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: -a flag accepted — passthrough still works with append flag.
		{
			Name:     "r2.1_append_flag_passthrough",
			Args:     []string{"-a"},
			Stdin:    []byte("append test\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.1: --append long flag accepted.
		{
			Name:     "r2.1_append_long_flag_passthrough",
			Args:     []string{"--append"},
			Stdin:    []byte("long append\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.2: -i flag accepted — passthrough with ignore-interrupts.
		{
			Name:     "r2.2_ignore_interrupts_passthrough",
			Args:     []string{"-i"},
			Stdin:    []byte("ignore int test\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.3: -a and -i combined — both flags work together.
		{
			Name:     "r2.3_combined_ai_flags",
			Args:     []string{"-ai"},
			Stdin:    []byte("combined\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R2.3: -i and -a as separate args.
		{
			Name:     "r2.3_separate_i_a_flags",
			Args:     []string{"-i", "-a"},
			Stdin:    []byte("separate flags\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.1: exit 0 when all writes succeed (explicit verification).
		{
			Name:     "r3.1_success_exit0",
			Stdin:    []byte("success\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.3: stdout still receives data when a file fails to open.
		// Two bad paths — stdout should still pass through all data.
		{
			Name:      "r3.3_stdout_continues_multiple_bad_files",
			Args:      []string{readOnlyPath, filepath.Join(dir, "also-bad", "x.txt")},
			Stdin:     []byte("still goes to stdout\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeTeeErrors},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestFileOutput verifies that tee writes correct file content. These tests
// run only the Go binary since both binaries in differential testing would
// write to the same file path, making file content checks unreliable.
func TestFileOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// R1.1, R1.3: single file — stdin written to stdout and file.
	t.Run("single_file_create", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		outFile := filepath.Join(outDir, "out.txt")
		input := []byte("hello\nworld\n")

		stdout, exitCode := runGoBin(t, goBin, []string{outFile}, input)
		if exitCode != 0 {
			t.Errorf("expected exit 0, got %d", exitCode)
		}
		if !bytes.Equal(stdout, input) {
			t.Errorf("stdout mismatch: got %q, want %q", stdout, input)
		}
		// R4.3: file content matches stdout byte-for-byte.
		assertFileContent(t, outFile, input)
	})

	// R1.1: multiple files — stdin written to stdout and all files.
	t.Run("multiple_files", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		fileA := filepath.Join(outDir, "a.txt")
		fileB := filepath.Join(outDir, "b.txt")
		input := []byte("line1\nline2\n")

		stdout, exitCode := runGoBin(t, goBin, []string{fileA, fileB}, input)
		if exitCode != 0 {
			t.Errorf("expected exit 0, got %d", exitCode)
		}
		if !bytes.Equal(stdout, input) {
			t.Errorf("stdout mismatch: got %q, want %q", stdout, input)
		}
		// R4.3: both files match stdout.
		assertFileContent(t, fileA, input)
		assertFileContent(t, fileB, input)
	})

	// R1.3: overwrite existing file — file is truncated before writing.
	t.Run("overwrite_existing", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		existingFile := filepath.Join(outDir, "existing.txt")
		if err := os.WriteFile(existingFile, []byte("old content that is longer\n"), 0o644); err != nil {
			t.Fatalf("writing existing file: %v", err)
		}
		input := []byte("new\n")

		_, exitCode := runGoBin(t, goBin, []string{existingFile}, input)
		if exitCode != 0 {
			t.Errorf("expected exit 0, got %d", exitCode)
		}
		assertFileContent(t, existingFile, input)
	})

	// R1.3: file created when it does not exist.
	t.Run("creates_nonexistent_file", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		newFile := filepath.Join(outDir, "brand-new.txt")
		input := []byte("created\n")

		_, exitCode := runGoBin(t, goBin, []string{newFile}, input)
		if exitCode != 0 {
			t.Errorf("expected exit 0, got %d", exitCode)
		}
		assertFileContent(t, newFile, input)
	})

	// R2.1: append mode preserves existing content and appends new data.
	t.Run("append_mode_preserves_existing", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		appendFile := filepath.Join(outDir, "append.txt")
		existing := []byte("old\n")
		if err := os.WriteFile(appendFile, existing, 0o644); err != nil {
			t.Fatalf("writing existing file: %v", err)
		}
		input := []byte("new\n")

		stdout, exitCode := runGoBin(t, goBin, []string{"-a", appendFile}, input)
		if exitCode != 0 {
			t.Errorf("expected exit 0, got %d", exitCode)
		}
		if !bytes.Equal(stdout, input) {
			t.Errorf("stdout mismatch: got %q, want %q", stdout, input)
		}
		// R2.1: file should contain old + new content.
		expected := append(existing, input...)
		assertFileContent(t, appendFile, expected)
	})

	// R2.1: append mode with --append long flag.
	t.Run("append_long_flag", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		appendFile := filepath.Join(outDir, "append-long.txt")
		existing := []byte("first\n")
		if err := os.WriteFile(appendFile, existing, 0o644); err != nil {
			t.Fatalf("writing existing file: %v", err)
		}
		input := []byte("second\n")

		_, exitCode := runGoBin(t, goBin, []string{"--append", appendFile}, input)
		if exitCode != 0 {
			t.Errorf("expected exit 0, got %d", exitCode)
		}
		expected := append(existing, input...)
		assertFileContent(t, appendFile, expected)
	})

	// R2.1: append mode creates file when it does not exist.
	t.Run("append_mode_creates_new_file", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		newFile := filepath.Join(outDir, "new-append.txt")
		input := []byte("appended to new\n")

		_, exitCode := runGoBin(t, goBin, []string{"-a", newFile}, input)
		if exitCode != 0 {
			t.Errorf("expected exit 0, got %d", exitCode)
		}
		assertFileContent(t, newFile, input)
	})

	// R2.1: append mode with multiple files.
	t.Run("append_mode_multiple_files", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		fileA := filepath.Join(outDir, "a.txt")
		fileB := filepath.Join(outDir, "b.txt")
		existingA := []byte("aaa\n")
		existingB := []byte("bbb\n")
		if err := os.WriteFile(fileA, existingA, 0o644); err != nil {
			t.Fatalf("writing file A: %v", err)
		}
		if err := os.WriteFile(fileB, existingB, 0o644); err != nil {
			t.Fatalf("writing file B: %v", err)
		}
		input := []byte("more\n")

		_, exitCode := runGoBin(t, goBin, []string{"-a", fileA, fileB}, input)
		if exitCode != 0 {
			t.Errorf("expected exit 0, got %d", exitCode)
		}
		assertFileContent(t, fileA, append(existingA, input...))
		assertFileContent(t, fileB, append(existingB, input...))
	})

	// R2.3: combined -ai flag preserves append behavior.
	t.Run("combined_ai_append_file", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		appendFile := filepath.Join(outDir, "ai.txt")
		existing := []byte("existing\n")
		if err := os.WriteFile(appendFile, existing, 0o644); err != nil {
			t.Fatalf("writing existing file: %v", err)
		}
		input := []byte("added\n")

		_, exitCode := runGoBin(t, goBin, []string{"-ai", appendFile}, input)
		if exitCode != 0 {
			t.Errorf("expected exit 0, got %d", exitCode)
		}
		expected := append(existing, input...)
		assertFileContent(t, appendFile, expected)
	})

	// R3.3: good file still receives all data when another file fails to open.
	t.Run("r3.3_good_file_with_bad_file", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		goodFile := filepath.Join(outDir, "good.txt")
		badFile := filepath.Join(outDir, "no-such-dir", "bad.txt")
		input := []byte("data for good file\n")

		stdout, exitCode := runGoBin(t, goBin, []string{badFile, goodFile}, input)
		if exitCode != 1 {
			t.Errorf("expected exit 1, got %d", exitCode)
		}
		// R3.3: stdout still receives all data.
		if !bytes.Equal(stdout, input) {
			t.Errorf("stdout mismatch: got %q, want %q", stdout, input)
		}
		// R3.3: good file receives all data despite bad file failure.
		assertFileContent(t, goodFile, input)
	})

	// R3.3: multiple good files with a bad file in between.
	t.Run("r3.3_good_files_around_bad_file", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		fileA := filepath.Join(outDir, "a.txt")
		fileB := filepath.Join(outDir, "b.txt")
		badFile := filepath.Join(outDir, "missing-dir", "bad.txt")
		input := []byte("resilient output\n")

		stdout, exitCode := runGoBin(t, goBin, []string{fileA, badFile, fileB}, input)
		if exitCode != 1 {
			t.Errorf("expected exit 1, got %d", exitCode)
		}
		if !bytes.Equal(stdout, input) {
			t.Errorf("stdout mismatch: got %q, want %q", stdout, input)
		}
		// R3.3: both good files receive all data.
		assertFileContent(t, fileA, input)
		assertFileContent(t, fileB, input)
	})
}

// TestDiffFileOutput runs both Go and reference binaries with file arguments
// in separate temp directories and compares their file output byte-for-byte.
//
// R4.1: Compares stdout output, file content, and exit codes between Go and ref.
// R4.2: Covers single file, multiple files, append mode, write-error handling.
// R4.3: Verifies file content matches stdout byte-for-byte.
func TestDiffFileOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGtee)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGtee, err)
	}

	// R4.2: single file — compare Go and ref file output.
	t.Run("single_file", func(t *testing.T) {
		t.Parallel()
		input := []byte("hello\nworld\n")

		goDir := t.TempDir()
		refDir := t.TempDir()
		goFile := filepath.Join(goDir, "out.txt")
		refFile := filepath.Join(refDir, "out.txt")

		goStdout, goExit := runGoBin(t, goBin, []string{goFile}, input)
		refStdout, refExit := runGoBin(t, refBin, []string{refFile}, input)

		compareResults(t, goStdout, refStdout, goExit, refExit)
		compareFileContent(t, goFile, refFile, "out.txt")
		// R4.3: file content matches stdout.
		assertFileContent(t, goFile, goStdout)
	})

	// R4.2: multiple files — compare Go and ref output for both files.
	t.Run("multiple_files", func(t *testing.T) {
		t.Parallel()
		input := []byte("line1\nline2\nline3\n")

		goDir := t.TempDir()
		refDir := t.TempDir()
		goFileA := filepath.Join(goDir, "a.txt")
		goFileB := filepath.Join(goDir, "b.txt")
		refFileA := filepath.Join(refDir, "a.txt")
		refFileB := filepath.Join(refDir, "b.txt")

		goStdout, goExit := runGoBin(t, goBin, []string{goFileA, goFileB}, input)
		refStdout, refExit := runGoBin(t, refBin, []string{refFileA, refFileB}, input)

		compareResults(t, goStdout, refStdout, goExit, refExit)
		compareFileContent(t, goFileA, refFileA, "a.txt")
		compareFileContent(t, goFileB, refFileB, "b.txt")
		// R4.3: both files match stdout.
		assertFileContent(t, goFileA, goStdout)
		assertFileContent(t, goFileB, goStdout)
	})

	// R4.2: append mode — both binaries append to existing files identically.
	t.Run("append_mode", func(t *testing.T) {
		t.Parallel()
		existing := []byte("old content\n")
		input := []byte("new content\n")

		goDir := t.TempDir()
		refDir := t.TempDir()
		goFile := filepath.Join(goDir, "append.txt")
		refFile := filepath.Join(refDir, "append.txt")

		if err := os.WriteFile(goFile, existing, 0o644); err != nil {
			t.Fatalf("writing go existing file: %v", err)
		}
		if err := os.WriteFile(refFile, existing, 0o644); err != nil {
			t.Fatalf("writing ref existing file: %v", err)
		}

		goStdout, goExit := runGoBin(t, goBin, []string{"-a", goFile}, input)
		refStdout, refExit := runGoBin(t, refBin, []string{"-a", refFile}, input)

		compareResults(t, goStdout, refStdout, goExit, refExit)
		compareFileContent(t, goFile, refFile, "append.txt")
	})

	// R4.2: write-error handling — both binaries exit 1 for bad path.
	t.Run("write_error", func(t *testing.T) {
		t.Parallel()
		input := []byte("data\n")

		goDir := t.TempDir()
		refDir := t.TempDir()
		goBadPath := filepath.Join(goDir, "no-such-dir", "file.txt")
		refBadPath := filepath.Join(refDir, "no-such-dir", "file.txt")

		goStdout, goExit := runGoBin(t, goBin, []string{goBadPath}, input)
		refStdout, refExit := runGoBin(t, refBin, []string{refBadPath}, input)

		// Both should exit 1 and still write stdin to stdout.
		compareResults(t, goStdout, refStdout, goExit, refExit)
	})

	// R4.2: write error with one good file — good file still gets output.
	t.Run("write_error_with_good_file", func(t *testing.T) {
		t.Parallel()
		input := []byte("partial success\n")

		goDir := t.TempDir()
		refDir := t.TempDir()
		goGood := filepath.Join(goDir, "good.txt")
		refGood := filepath.Join(refDir, "good.txt")
		goBad := filepath.Join(goDir, "no-dir", "bad.txt")
		refBad := filepath.Join(refDir, "no-dir", "bad.txt")

		goStdout, goExit := runGoBin(t, goBin, []string{goBad, goGood}, input)
		refStdout, refExit := runGoBin(t, refBin, []string{refBad, refGood}, input)

		compareResults(t, goStdout, refStdout, goExit, refExit)
		compareFileContent(t, goGood, refGood, "good.txt")
		// R4.3: good file matches stdout despite bad file failure.
		assertFileContent(t, goGood, goStdout)
	})
}

// TestSIGINTSuppression verifies that tee -i continues reading after receiving
// SIGINT, matching GNU tee behavior.
//
// R4.2: SIGINT suppression (-i with a signal sent during operation).
func TestSIGINTSuppression(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "sigint.txt")

	cmd := exec.Command(goBin, "-i", outFile)
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("starting tee: %v", err)
	}

	// Write first chunk.
	firstChunk := []byte("before signal\n")
	if _, err := stdinPipe.Write(firstChunk); err != nil {
		t.Fatalf("writing first chunk: %v", err)
	}

	// Read first chunk from stdout to confirm tee is running and has
	// set up its signal handlers.
	buf := make([]byte, len(firstChunk))
	if _, err := io.ReadFull(stdoutPipe, buf); err != nil {
		t.Fatalf("reading first chunk from stdout: %v", err)
	}

	// Send SIGINT — tee -i should ignore it and continue.
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("sending SIGINT: %v", err)
	}

	// Write second chunk after SIGINT — tee should still be running.
	secondChunk := []byte("after signal\n")
	if _, err := stdinPipe.Write(secondChunk); err != nil {
		t.Fatalf("writing second chunk: %v", err)
	}

	// Read second chunk from stdout to confirm tee processed it.
	buf2 := make([]byte, len(secondChunk))
	if _, err := io.ReadFull(stdoutPipe, buf2); err != nil {
		t.Fatalf("reading second chunk from stdout: %v", err)
	}

	// Close stdin to signal EOF.
	if err := stdinPipe.Close(); err != nil {
		t.Fatalf("closing stdin: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		t.Errorf("tee -i exited with error after SIGINT: %v", err)
	}

	expected := []byte("before signal\nafter signal\n")

	// Verify stdout contains both chunks.
	stdout := append(buf, buf2...)
	if !bytes.Equal(stdout, expected) {
		t.Errorf("stdout mismatch: got %q, want %q", stdout, expected)
	}

	// R4.3: file content matches stdout.
	assertFileContent(t, outFile, expected)
}

// compareResults checks that Go and reference binary produced the same stdout
// and exit code.
func compareResults(t *testing.T, goStdout, refStdout []byte, goExit, refExit int) {
	t.Helper()
	if goExit != refExit {
		t.Errorf("exit code mismatch: go=%d ref=%d", goExit, refExit)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\n  go:  %q\n  ref: %q", goStdout, refStdout)
	}
}

// compareFileContent reads the files at goPath and refPath and verifies their
// contents are byte-for-byte identical.
func compareFileContent(t *testing.T, goPath, refPath, label string) {
	t.Helper()
	goContent, err := os.ReadFile(goPath)
	if err != nil {
		t.Fatalf("reading go file %s: %v", label, err)
	}
	refContent, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("reading ref file %s: %v", label, err)
	}
	if !bytes.Equal(goContent, refContent) {
		t.Errorf("file %s content mismatch:\n  go:  %q\n  ref: %q", label, goContent, refContent)
	}
}

// runGoBin executes the Go tee binary with the given args and stdin,
// returning captured stdout and exit code.
func runGoBin(t *testing.T, binary string, args []string, stdin []byte) (stdout []byte, exitCode int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run binary %q: %v", binary, err)
		}
	}
	return outBuf.Bytes(), exitCode
}

// assertFileContent reads the file at path and compares its content to expected.
func assertFileContent(t *testing.T, path string, expected []byte) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !bytes.Equal(content, expected) {
		t.Errorf("file %s content mismatch: got %q, want %q", path, content, expected)
	}
}
