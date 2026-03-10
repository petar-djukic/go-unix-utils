// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests: prd007-sponge R1.1–R1.4, R2.4–R2.5, R3.1–R3.3, R4.1–R4.3,
// R5.1–R5.4, R6.1–R6.2 via differential testing against sponge (Homebrew moreutils).
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skipf("reference binary sponge not in PATH: %v", err)
	}

	dir := t.TempDir()

	// R1.3: stdin to stdout (passthrough mode, no filename).
	t.Run("stdin_to_stdout", func(t *testing.T) {
		tests := []testutils.DiffTest{
			{
				Name:    "small_stdin_to_stdout",
				Stdin:   []byte("hello\n"),
				WorkDir: dir,
			},
			{
				Name:    "empty_stdin_to_stdout",
				Stdin:   []byte{},
				WorkDir: dir,
			},
			{
				Name:    "multiline_stdin_to_stdout",
				Stdin:   []byte("line1\nline2\nline3\n"),
				WorkDir: dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.1, R1.2: stdin to file.
	t.Run("stdin_to_file", func(t *testing.T) {
		outPath := filepath.Join(dir, "out_basic.txt")
		tests := []testutils.DiffTest{
			{
				Name:          "small_stdin_to_file",
				Args:          []string{outPath},
				Stdin:         []byte("hello\n"),
				WorkDir:       dir,
				ExpectedFiles: map[string][]byte{outPath: []byte("hello\n")},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.1: same-file pipeline (the core sponge use case).
	t.Run("same_file_pipeline", func(t *testing.T) {
		// Create a file, read it with cat, pipe through sponge back to same file.
		// This verifies sponge reads ALL stdin before opening the output.
		srcPath := filepath.Join(dir, "same_file.txt")
		content := "alpha\nbeta\ngamma\n"

		// Test with Go binary.
		if err := os.WriteFile(srcPath, []byte(content), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		goCmd := exec.Command("sh", "-c", "cat "+srcPath+" | "+goBin+" "+srcPath)
		goCmd.Dir = dir
		goOut, goErr := goCmd.CombinedOutput()
		if goErr != nil {
			t.Fatalf("go binary failed: %v\n%s", goErr, goOut)
		}
		goResult, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("read after go: %v", err)
		}

		// Test with reference binary.
		if err := os.WriteFile(srcPath, []byte(content), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		refCmd := exec.Command("sh", "-c", "cat "+srcPath+" | "+refBin+" "+srcPath)
		refCmd.Dir = dir
		refOut, refErr := refCmd.CombinedOutput()
		if refErr != nil {
			t.Fatalf("ref binary failed: %v\n%s", refErr, refOut)
		}
		refResult, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("read after ref: %v", err)
		}

		if !bytes.Equal(goResult, refResult) {
			t.Errorf("same-file pipeline divergence:\ngo:  %q\nref: %q", goResult, refResult)
		}
		if string(goResult) != content {
			t.Errorf("same-file pipeline: expected %q, got %q", content, goResult)
		}
	})

	// R1.1, R1.2: overwrite existing file.
	t.Run("overwrite_existing", func(t *testing.T) {
		outPath := filepath.Join(dir, "out_overwrite.txt")
		// Create existing file with different content.
		if err := os.WriteFile(outPath, []byte("old content\n"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		tests := []testutils.DiffTest{
			{
				Name:          "overwrite_existing_file",
				Args:          []string{outPath},
				Stdin:         []byte("new content\n"),
				WorkDir:       dir,
				ExpectedFiles: map[string][]byte{outPath: []byte("new content\n")},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.1: large input to verify buffering works.
	t.Run("large_input", func(t *testing.T) {
		outPath := filepath.Join(dir, "out_large.txt")
		// Generate ~100KB of data.
		var largeInput strings.Builder
		for range 10000 {
			largeInput.WriteString("this is line number for large input test\n")
		}
		data := []byte(largeInput.String())
		tests := []testutils.DiffTest{
			{
				Name:          "large_stdin_to_file",
				Args:          []string{outPath},
				Stdin:         data,
				WorkDir:       dir,
				ExpectedFiles: map[string][]byte{outPath: data},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.3: large input to stdout.
	t.Run("large_input_stdout", func(t *testing.T) {
		var largeInput strings.Builder
		for range 5000 {
			largeInput.WriteString("stdout large input line\n")
		}
		data := []byte(largeInput.String())
		tests := []testutils.DiffTest{
			{
				Name:    "large_stdin_to_stdout",
				Stdin:   data,
				WorkDir: dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R3.1: append mode with existing file.
	t.Run("append_existing_file", func(t *testing.T) {
		original := []byte("original\n")
		appended := []byte("appended\n")
		expected := append(original, appended...)

		runAppend := func(t *testing.T, binary, outPath string) []byte {
			t.Helper()
			if err := os.WriteFile(outPath, original, 0o644); err != nil {
				t.Fatalf("setup: %v", err)
			}
			cmd := exec.Command(binary, "-a", outPath)
			cmd.Stdin = bytes.NewReader(appended)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("binary failed: %v\n%s", err, out)
			}
			result, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("read result: %v", err)
			}
			return result
		}

		goResult := runAppend(t, goBin, filepath.Join(dir, "append_go.txt"))
		refResult := runAppend(t, refBin, filepath.Join(dir, "append_ref.txt"))
		if !bytes.Equal(goResult, refResult) {
			t.Errorf("append divergence:\ngo:  %q\nref: %q", goResult, refResult)
		}
		if !bytes.Equal(goResult, expected) {
			t.Errorf("append: expected %q, got %q", expected, goResult)
		}
	})

	// R3.2: append mode with non-existing file creates new file.
	t.Run("append_new_file", func(t *testing.T) {
		content := []byte("new content\n")

		runAppendNew := func(t *testing.T, binary, outPath string) []byte {
			t.Helper()
			os.Remove(outPath) // best-effort ensure not exists
			cmd := exec.Command(binary, "-a", outPath)
			cmd.Stdin = bytes.NewReader(content)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("binary failed: %v\n%s", err, out)
			}
			result, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("read result: %v", err)
			}
			return result
		}

		goResult := runAppendNew(t, goBin, filepath.Join(dir, "append_new_go.txt"))
		refResult := runAppendNew(t, refBin, filepath.Join(dir, "append_new_ref.txt"))
		if !bytes.Equal(goResult, refResult) {
			t.Errorf("append new file divergence:\ngo:  %q\nref: %q", goResult, refResult)
		}
		if !bytes.Equal(goResult, content) {
			t.Errorf("append new file: expected %q, got %q", content, goResult)
		}
	})

	// R3.1/R2.3: permission preservation when overwriting an existing file.
	t.Run("permission_preservation", func(t *testing.T) {
		runAndCheckPerms := func(t *testing.T, binary, outPath string, mode os.FileMode) os.FileMode {
			t.Helper()
			if err := os.WriteFile(outPath, []byte("old\n"), mode); err != nil {
				t.Fatalf("setup: %v", err)
			}
			cmd := exec.Command(binary, outPath)
			cmd.Stdin = strings.NewReader("new\n")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("binary %s failed: %v\n%s", binary, err, out)
			}
			info, err := os.Stat(outPath)
			if err != nil {
				t.Fatalf("stat after write: %v", err)
			}
			return info.Mode().Perm()
		}

		// R3.1: file with 0600 permissions should retain 0600 after sponge.
		goPerms := runAndCheckPerms(t, goBin, filepath.Join(dir, "perms_go_600.txt"), 0o600)
		refPerms := runAndCheckPerms(t, refBin, filepath.Join(dir, "perms_ref_600.txt"), 0o600)
		if goPerms != refPerms {
			t.Errorf("permission divergence (0600): go=%o ref=%o", goPerms, refPerms)
		}
		if goPerms != 0o600 {
			t.Errorf("expected permissions 0600, got %o", goPerms)
		}

		// R3.1: file with 0755 permissions should retain 0755 after sponge.
		goPerms = runAndCheckPerms(t, goBin, filepath.Join(dir, "perms_go_755.txt"), 0o755)
		refPerms = runAndCheckPerms(t, refBin, filepath.Join(dir, "perms_ref_755.txt"), 0o755)
		if goPerms != refPerms {
			t.Errorf("permission divergence (0755): go=%o ref=%o", goPerms, refPerms)
		}
		if goPerms != 0o755 {
			t.Errorf("expected permissions 0755, got %o", goPerms)
		}
	})

	// R3.2: new file gets default permissions (0666 & ~umask).
	t.Run("default_permissions_new_file", func(t *testing.T) {
		goPath := filepath.Join(dir, "out_new_perms_go.txt")
		refPath := filepath.Join(dir, "out_new_perms_ref.txt")

		os.Remove(goPath) // best-effort cleanup
		os.Remove(refPath) // best-effort cleanup

		// Write with Go binary.
		goCmd := exec.Command(goBin, goPath)
		goCmd.Stdin = strings.NewReader("test\n")
		goCmd.Dir = dir
		if out, err := goCmd.CombinedOutput(); err != nil {
			t.Fatalf("go binary failed: %v\n%s", err, out)
		}
		goInfo, err := os.Stat(goPath)
		if err != nil {
			t.Fatalf("stat go output: %v", err)
		}

		// Write with reference binary.
		refCmd := exec.Command(refBin, refPath)
		refCmd.Stdin = strings.NewReader("test\n")
		refCmd.Dir = dir
		if out, err := refCmd.CombinedOutput(); err != nil {
			t.Fatalf("ref binary failed: %v\n%s", err, out)
		}
		refInfo, err := os.Stat(refPath)
		if err != nil {
			t.Fatalf("stat ref output: %v", err)
		}

		goPerms := goInfo.Mode().Perm()
		refPerms := refInfo.Mode().Perm()
		if goPerms != refPerms {
			t.Errorf("default permission divergence: go=%o ref=%o", goPerms, refPerms)
		}
	})

	// R2.5: error on unwritable output path — both binaries must exit non-zero.
	t.Run("unwritable_output", func(t *testing.T) {
		// Create a read-only directory to prevent file creation.
		roDir := filepath.Join(dir, "readonly")
		if err := os.Mkdir(roDir, 0o555); err != nil {
			t.Fatalf("setup: %v", err)
		}
		t.Cleanup(func() {
			os.Chmod(roDir, 0o755) // best-effort restore for cleanup
		})

		runBin := func(t *testing.T, binary, outPath string) int {
			t.Helper()
			cmd := exec.Command(binary, outPath)
			cmd.Stdin = strings.NewReader("data\n")
			cmd.Dir = dir
			err := cmd.Run()
			if err == nil {
				return 0
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode()
			}
			t.Fatalf("unexpected error: %v", err)
			return -1
		}

		goExit := runBin(t, goBin, filepath.Join(roDir, "cannot_create.txt"))
		refExit := runBin(t, refBin, filepath.Join(roDir, "cannot_create_ref.txt"))
		if goExit != refExit {
			t.Errorf("exit code divergence: go=%d ref=%d", goExit, refExit)
		}
		if goExit != 1 {
			t.Errorf("expected exit code 1, got %d", goExit)
		}
	})

	// R3.1: append mode preserves existing file content with multiline input.
	t.Run("append_multiline", func(t *testing.T) {
		original := []byte("line1\nline2\n")
		addition := []byte("line3\nline4\n")
		expected := append(original, addition...)

		runAppend := func(t *testing.T, binary, outPath string) []byte {
			t.Helper()
			if err := os.WriteFile(outPath, original, 0o644); err != nil {
				t.Fatalf("setup: %v", err)
			}
			cmd := exec.Command(binary, "-a", outPath)
			cmd.Stdin = bytes.NewReader(addition)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("binary failed: %v\n%s", err, out)
			}
			result, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("read result: %v", err)
			}
			return result
		}

		goResult := runAppend(t, goBin, filepath.Join(dir, "append_multi_go.txt"))
		refResult := runAppend(t, refBin, filepath.Join(dir, "append_multi_ref.txt"))
		if !bytes.Equal(goResult, refResult) {
			t.Errorf("append multiline divergence:\ngo:  %q\nref: %q", goResult, refResult)
		}
		if !bytes.Equal(goResult, expected) {
			t.Errorf("append multiline: expected %q, got %q", expected, goResult)
		}
	})

	// R3.3: same-file append pipeline — cat file | sponge -a file must
	// read the original content before overwriting, producing [original][stdin].
	t.Run("same_file_append_pipeline", func(t *testing.T) {
		original := "first\nsecond\n"

		runPipeline := func(t *testing.T, binary, path string) []byte {
			t.Helper()
			if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
				t.Fatalf("setup: %v", err)
			}
			// Pipe the file's own content back through sponge -a to the same file.
			// Result should be [original][original] = two copies of the content.
			cmd := exec.Command("sh", "-c", "cat "+path+" | "+binary+" -a "+path)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("binary failed: %v\n%s", err, out)
			}
			result, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read result: %v", err)
			}
			return result
		}

		goResult := runPipeline(t, goBin, filepath.Join(dir, "same_append_go.txt"))
		refResult := runPipeline(t, refBin, filepath.Join(dir, "same_append_ref.txt"))
		if !bytes.Equal(goResult, refResult) {
			t.Errorf("same-file append divergence:\ngo:  %q\nref: %q", goResult, refResult)
		}
		expected := original + original
		if string(goResult) != expected {
			t.Errorf("same-file append: expected %q, got %q", expected, goResult)
		}
	})

	// R4.1, R4.3: passthrough mode with binary data — buffered in memory,
	// written to stdout without temp file.
	t.Run("passthrough_binary_data", func(t *testing.T) {
		// Create binary data with null bytes and high bytes.
		data := make([]byte, 256)
		for i := range data {
			data[i] = byte(i)
		}
		tests := []testutils.DiffTest{
			{
				Name:    "binary_data_to_stdout",
				Stdin:   data,
				WorkDir: dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R4.1, R4.2: passthrough mode with large input verifies that large
	// data is correctly written to stdout.
	t.Run("passthrough_large", func(t *testing.T) {
		// Generate ~1MB of data to exercise the large-input passthrough path.
		var large strings.Builder
		for range 50000 {
			large.WriteString("passthrough large input line\n")
		}
		data := []byte(large.String())
		tests := []testutils.DiffTest{
			{
				Name:    "large_passthrough_1mb",
				Stdin:   data,
				WorkDir: dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R3.3: append mode preserves permissions of the original file.
	t.Run("append_permission_preservation", func(t *testing.T) {
		runAndCheckPerms := func(t *testing.T, binary, outPath string, mode os.FileMode) os.FileMode {
			t.Helper()
			if err := os.WriteFile(outPath, []byte("old\n"), mode); err != nil {
				t.Fatalf("setup: %v", err)
			}
			cmd := exec.Command(binary, "-a", outPath)
			cmd.Stdin = strings.NewReader("new\n")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("binary %s failed: %v\n%s", binary, err, out)
			}
			info, err := os.Stat(outPath)
			if err != nil {
				t.Fatalf("stat after write: %v", err)
			}
			return info.Mode().Perm()
		}

		goPerms := runAndCheckPerms(t, goBin, filepath.Join(dir, "append_perms_go.txt"), 0o600)
		refPerms := runAndCheckPerms(t, refBin, filepath.Join(dir, "append_perms_ref.txt"), 0o600)
		if goPerms != refPerms {
			t.Errorf("append permission divergence (0600): go=%o ref=%o", goPerms, refPerms)
		}
	})

	// R5.1: both binaries exit 0 on successful file write.
	t.Run("exit_zero_on_success", func(t *testing.T) {
		outPath := filepath.Join(dir, "exit0_test.txt")
		tests := []testutils.DiffTest{
			{
				Name:     "exit_zero_file_write",
				Args:     []string{outPath},
				Stdin:    []byte("success\n"),
				WorkDir:  dir,
				ExitCode: 0,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R5.1: exit 0 on successful passthrough to stdout.
	t.Run("exit_zero_passthrough", func(t *testing.T) {
		tests := []testutils.DiffTest{
			{
				Name:     "exit_zero_stdout",
				Stdin:    []byte("passthrough ok\n"),
				WorkDir:  dir,
				ExitCode: 0,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R5.2: both binaries exit 1 when the output directory does not exist.
	// The temp file cannot be created, triggering a descriptive error on stderr.
	t.Run("exit_one_nonexistent_dir", func(t *testing.T) {
		badPath := filepath.Join(dir, "no_such_dir", "file.txt")

		runBin := func(t *testing.T, binary string) (int, string) {
			t.Helper()
			cmd := exec.Command(binary, badPath)
			cmd.Stdin = strings.NewReader("data\n")
			cmd.Dir = dir
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			if err == nil {
				return 0, stderr.String()
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode(), stderr.String()
			}
			t.Fatalf("unexpected error: %v", err)
			return -1, ""
		}

		goExit, goStderr := runBin(t, goBin)
		refExit, _ := runBin(t, refBin)
		if goExit != refExit {
			t.Errorf("exit code divergence: go=%d ref=%d", goExit, refExit)
		}
		if goExit != 1 {
			t.Errorf("expected exit code 1, got %d", goExit)
		}
		// R5.2: verify descriptive error message on stderr.
		if goStderr == "" {
			t.Error("expected error message on stderr, got empty")
		}
	})

	// R5.3: verify the binary exits cleanly (no panic stack trace) on error.
	// Go's OOM produces an unrecoverable runtime crash, but the recover handler
	// catches other panics. This test verifies stderr contains a descriptive
	// message without a Go stack trace.
	t.Run("no_panic_on_error", func(t *testing.T) {
		badPath := filepath.Join(dir, "missing_parent", "deep", "file.txt")

		cmd := exec.Command(goBin, badPath)
		cmd.Stdin = strings.NewReader("data\n")
		cmd.Dir = dir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected nonzero exit, got 0")
		}
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("unexpected error type: %v", err)
		}
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
		}
		// R5.3: stderr must not contain Go panic/goroutine stack traces.
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "goroutine") {
			t.Errorf("go binary panicked (goroutine trace in stderr):\n%s", stderrStr)
		}
		if strings.Contains(stderrStr, "panic:") {
			t.Errorf("go binary panicked (panic: in stderr):\n%s", stderrStr)
		}
	})

	// R5.4: verify temp file cleanup after error — no sponge.* files
	// should remain in the output directory after a failed write.
	t.Run("temp_file_cleanup_on_error", func(t *testing.T) {
		cleanupDir := filepath.Join(dir, "cleanup_check")
		if err := os.Mkdir(cleanupDir, 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}

		// Create a directory as the output target — rename over a directory
		// fails, and os.Create on a directory also fails, so sponge exits 1.
		// The temp file is created in filepath.Dir(outputAsDir) = cleanupDir.
		outputAsDir := filepath.Join(cleanupDir, "is_a_dir")
		if err := os.Mkdir(outputAsDir, 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}

		checkCleanup := func(t *testing.T, binary, label string) {
			t.Helper()
			cmd := exec.Command(binary, outputAsDir)
			cmd.Stdin = strings.NewReader("data\n")
			cmd.Dir = cleanupDir
			cmd.Run() // ignore error — we expect failure
			matches, _ := filepath.Glob(filepath.Join(cleanupDir, "sponge.*"))
			if len(matches) > 0 {
				t.Errorf("%s left temp file(s): %v", label, matches)
			}
		}

		checkCleanup(t, goBin, "go binary")
		checkCleanup(t, refBin, "ref binary")
	})

	// R5.4: verify temp file cleanup on successful write — no leftover
	// sponge.* files remain after a normal operation.
	t.Run("temp_file_cleanup_on_success", func(t *testing.T) {
		cleanupDir := filepath.Join(dir, "cleanup_success")
		if err := os.Mkdir(cleanupDir, 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}

		outPath := filepath.Join(cleanupDir, "output.txt")
		cmd := exec.Command(goBin, outPath)
		cmd.Stdin = strings.NewReader("data\n")
		cmd.Dir = cleanupDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go binary failed: %v\n%s", err, out)
		}
		matches, _ := filepath.Glob(filepath.Join(cleanupDir, "sponge.*"))
		if len(matches) > 0 {
			t.Errorf("go binary left temp file(s) after success: %v", matches)
		}
	})

	// R6.1: --help prints usage to stdout and exits 0.
	t.Run("help_flag", func(t *testing.T) {
		runHelp := func(t *testing.T, binary, label string) (int, string, string) {
			t.Helper()
			cmd := exec.Command(binary, "--help")
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					t.Fatalf("%s: unexpected error: %v", label, err)
				}
			}
			return exitCode, stdout.String(), stderr.String()
		}

		goExit, goStdout, _ := runHelp(t, goBin, "go")
		refExit, _, _ := runHelp(t, refBin, "ref")

		// Both must exit 0.
		if goExit != refExit {
			t.Errorf("--help exit code divergence: go=%d ref=%d", goExit, refExit)
		}
		if goExit != 0 {
			t.Errorf("--help expected exit 0, got %d", goExit)
		}
		// Go binary must produce non-empty stdout.
		if goStdout == "" {
			t.Error("--help produced no stdout output")
		}
	})

	// R6.2: --version prints version to stdout and exits 0.
	t.Run("version_flag", func(t *testing.T) {
		runVersion := func(t *testing.T, binary, label string) (int, string, string) {
			t.Helper()
			cmd := exec.Command(binary, "--version")
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					t.Fatalf("%s: unexpected error: %v", label, err)
				}
			}
			return exitCode, stdout.String(), stderr.String()
		}

		goExit, goStdout, _ := runVersion(t, goBin, "go")
		refExit, _, _ := runVersion(t, refBin, "ref")

		// Both must exit 0.
		if goExit != refExit {
			t.Errorf("--version exit code divergence: go=%d ref=%d", goExit, refExit)
		}
		if goExit != 0 {
			t.Errorf("--version expected exit 0, got %d", goExit)
		}
		// Go binary must produce non-empty stdout.
		if goStdout == "" {
			t.Error("--version produced no stdout output")
		}
	})
}
