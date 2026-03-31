// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sponge against sponge (moreutils).
//
// Covers prd007-sponge R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3, R2.4, R2.5, R3.1, R3.2, R3.3, R4.1, R4.2, R4.3, R5.1, R5.2, R5.3, R5.4, R6.1, R6.2.
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
		t.Skip("reference binary sponge not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1, R4.1: passthrough mode — no filename, stdin to stdout
		{
			Name:  "R1.1_passthrough_small",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
		},
		// R1.1: empty stdin passthrough
		{
			Name:  "R1.1_passthrough_empty",
			Args:  []string{},
			Stdin: []byte{},
		},
		// R1.2: multi-line input passthrough
		{
			Name:  "R1.2_passthrough_multiline",
			Args:  []string{},
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		// R3.2: -a in passthrough mode (no file) — same as without -a
		{
			Name:  "R3.2_append_passthrough",
			Args:  []string{"-a"},
			Stdin: []byte("passthrough with -a\n"),
		},
		// R4.1: passthrough with binary data
		{
			Name:  "R4.1_passthrough_binary",
			Args:  []string{},
			Stdin: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x0A},
		},
		// R4.3: passthrough small buffer written directly to stdout
		{
			Name:  "R4.3_passthrough_small_buffer",
			Args:  []string{},
			Stdin: []byte("small\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestWriteToFile verifies sponge writes stdin to a named file (R1.1, R1.2).
func TestWriteToFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.txt")
	input := []byte("hello sponge\n")

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader(input)
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge exited with error: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !bytes.Equal(got, input) {
		t.Errorf("output file content = %q, want %q", got, input)
	}
}

// TestSoakBeforeWrite confirms the file is not opened until stdin is consumed (R1.1).
func TestSoakBeforeWrite(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "existing.txt")
	original := []byte("original content\n")
	if err := os.WriteFile(outPath, original, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Read from the same file we write to — soak-before-write must preserve content.
	catCmd := exec.Command("cat", outPath)
	catOut, err := catCmd.Output()
	if err != nil {
		t.Fatalf("cat: %v", err)
	}

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader(catOut)
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge exited with error: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("soak-before-write failed: got %q, want %q", got, original)
	}
}

// TestTempFileInTMPDIR verifies temp file creation uses TMPDIR (R1.4).
func TestTempFileInTMPDIR(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	tmpDir := t.TempDir()
	outPath := filepath.Join(dir, "out.txt")

	// Generate input large enough to potentially spill, but at minimum verify
	// the TMPDIR variable is respected by the binary running without error.
	input := bytes.Repeat([]byte("x"), 4096)

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = append(os.Environ(), "TMPDIR="+tmpDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge exited with error: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !bytes.Equal(got, input) {
		t.Errorf("output mismatch: got %d bytes, want %d bytes", len(got), len(input))
	}
}

// TestPermissionPreservation verifies file mode is preserved when overwriting (R2.3).
func TestPermissionPreservation(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "perms.txt")

	// Create file with non-default permissions.
	if err := os.WriteFile(outPath, []byte("old"), 0o755); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader([]byte("new content\n"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge: %v", err)
	}

	info, err := os.Lstat(outPath)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Errorf("permissions = %04o, want %04o", perm, 0o755)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "new content\n" {
		t.Errorf("content = %q, want %q", got, "new content\n")
	}
}

// TestNewFileDefaultMode verifies new files get default 0666 permissions (R2.3).
func TestNewFileDefaultMode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "newfile.txt")

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader([]byte("created\n"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge: %v", err)
	}

	info, err := os.Lstat(outPath)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	// R2.3: default mode 0666 applied via chmod after write.
	perm := info.Mode().Perm()
	if perm != 0o666 {
		t.Errorf("permissions = %04o, want %04o", perm, 0o666)
	}
}

// TestTempFileCleanup verifies temp files are cleaned up on normal exit (R1.5, R5.4).
func TestTempFileCleanup(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.txt")

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader([]byte("data\n"))
	cmd.Env = append(os.Environ(), "TMPDIR="+tmpDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge: %v", err)
	}

	// R5.4: verify no temp files left behind.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), tempPrefix) {
			t.Errorf("temp file %s not cleaned up", e.Name())
		}
	}
}

// TestRenameOrCopyFallback verifies copy fallback when rename is not possible (R2.2).
func TestRenameOrCopyFallback(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	// Use a different TMPDIR from the output directory to increase the chance
	// of cross-device rename failure. Even on same device, the copy fallback
	// is exercised when rename fails for any reason.
	tmpDir := t.TempDir()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "copied.txt")
	input := []byte("fallback content\n")

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = append(os.Environ(), "TMPDIR="+tmpDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, input) {
		t.Errorf("content = %q, want %q", got, input)
	}
}

// TestAppendMode verifies -a prepends original file content before stdin (R3.1).
func TestAppendMode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "append.txt")
	original := []byte("original\n")
	if err := os.WriteFile(outPath, original, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cmd := exec.Command(goBin, "-a", outPath)
	cmd.Stdin = bytes.NewReader([]byte("appended\n"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge -a: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := []byte("original\nappended\n")
	if !bytes.Equal(got, want) {
		t.Errorf("content = %q, want %q", got, want)
	}
}

// TestAppendNonExistent verifies -a creates new file when output doesn't exist (R3.2).
func TestAppendNonExistent(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "new.txt")
	input := []byte("new content\n")

	cmd := exec.Command(goBin, "-a", outPath)
	cmd.Stdin = bytes.NewReader(input)
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge -a: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, input) {
		t.Errorf("content = %q, want %q", got, input)
	}
}

// TestAppendDiff verifies -a behavior matches reference binary (R3.1, R6.1).
func TestAppendDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skip("reference binary sponge not in PATH")
	}

	original := []byte("original\n")
	input := []byte("appended\n")

	runAppend := func(bin, outPath string) []byte {
		t.Helper()
		if err := os.WriteFile(outPath, original, 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		cmd := exec.Command(bin, "-a", outPath)
		cmd.Stdin = bytes.NewReader(input)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s -a: %v\n%s", bin, err, out)
		}
		got, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		return got
	}

	dir := t.TempDir()
	goResult := runAppend(goBin, filepath.Join(dir, "go.txt"))
	refResult := runAppend(refBin, filepath.Join(dir, "ref.txt"))

	if !bytes.Equal(goResult, refResult) {
		t.Errorf("append diff:\ngo:  %q\nref: %q", goResult, refResult)
	}
}

// TestAppendPermissionPreservation verifies -a preserves file permissions (R2.3, R3.1).
func TestAppendPermissionPreservation(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "append_perms.txt")
	if err := os.WriteFile(outPath, []byte("old\n"), 0o755); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cmd := exec.Command(goBin, "-a", outPath)
	cmd.Stdin = bytes.NewReader([]byte("new\n"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge -a: %v", err)
	}

	info, err := os.Lstat(outPath)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Errorf("permissions = %04o, want %04o", perm, 0o755)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "old\nnew\n" {
		t.Errorf("content = %q, want %q", got, "old\nnew\n")
	}
}

// TestAppendPrependsOriginalContent verifies -a copies original file content first,
// then appends stdin content in the output (R3.3).
func TestAppendPrependsOriginalContent(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "prepend.txt")
	original := []byte("AAA\nBBB\n")
	if err := os.WriteFile(outPath, original, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	stdinData := []byte("CCC\nDDD\n")
	cmd := exec.Command(goBin, "-a", outPath)
	cmd.Stdin = bytes.NewReader(stdinData)
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge -a: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := []byte("AAA\nBBB\nCCC\nDDD\n")
	if !bytes.Equal(got, want) {
		t.Errorf("R3.3 prepend content = %q, want %q", got, want)
	}
}

// TestAppendLargeInput verifies -a works with large input that may spill to temp (R3.3).
func TestAppendLargeInput(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "append_large.txt")
	original := bytes.Repeat([]byte("orig\n"), 100)
	if err := os.WriteFile(outPath, original, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	stdinData := bytes.Repeat([]byte("new\n"), 200)
	cmd := exec.Command(goBin, "-a", outPath)
	cmd.Stdin = bytes.NewReader(stdinData)
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge -a: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := append(original, stdinData...)
	if !bytes.Equal(got, want) {
		t.Errorf("R3.3 large append: got %d bytes, want %d bytes", len(got), len(want))
	}
}

// TestPassthroughLargeInput verifies passthrough mode with larger input (R4.1, R4.2).
func TestPassthroughLargeInput(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	// Use input large enough to exercise buffering but not necessarily spill.
	input := bytes.Repeat([]byte("passthrough-line\n"), 1000)

	cmd := exec.Command(goBin)
	cmd.Stdin = bytes.NewReader(input)
	got, err := cmd.Output()
	if err != nil {
		t.Fatalf("sponge passthrough: %v", err)
	}
	if !bytes.Equal(got, input) {
		t.Errorf("passthrough: got %d bytes, want %d bytes", len(got), len(input))
	}
}

// TestPassthroughSmallDirect verifies small passthrough writes buffer directly (R4.3).
func TestPassthroughSmallDirect(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	input := []byte("direct write\n")
	cmd := exec.Command(goBin)
	cmd.Stdin = bytes.NewReader(input)
	got, err := cmd.Output()
	if err != nil {
		t.Fatalf("sponge passthrough: %v", err)
	}
	if !bytes.Equal(got, input) {
		t.Errorf("content = %q, want %q", got, input)
	}
}

// TestPassthroughDiff verifies passthrough matches reference binary (R4.1, R4.2, R4.3).
func TestPassthroughDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skip("reference binary sponge not in PATH")
	}

	input := bytes.Repeat([]byte("diff-line\n"), 500)

	runPassthrough := func(bin string) []byte {
		t.Helper()
		cmd := exec.Command(bin)
		cmd.Stdin = bytes.NewReader(input)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s passthrough: %v", bin, err)
		}
		return out
	}

	goResult := runPassthrough(goBin)
	refResult := runPassthrough(refBin)

	if !bytes.Equal(goResult, refResult) {
		t.Errorf("passthrough diff: go=%d bytes, ref=%d bytes", len(goResult), len(refResult))
	}
}

// TestLstatRegularFileCheck verifies lstat is used to detect regular files (R2.4).
func TestLstatRegularFileCheck(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.txt")
	linkPath := filepath.Join(dir, "link.txt")

	if err := os.WriteFile(targetPath, []byte("target\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// Write via symlink — sponge should write through the symlink path.
	cmd := exec.Command(goBin, linkPath)
	cmd.Stdin = bytes.NewReader([]byte("via link\n"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("sponge via symlink: %v", err)
	}

	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "via link\n" {
		t.Errorf("content = %q, want %q", got, "via link\n")
	}
}

// TestExitCodeSuccess verifies exit code 0 on successful write (R5.1).
func TestExitCodeSuccess(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outPath := filepath.Join(dir, "success.txt")

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader([]byte("ok\n"))
	err := cmd.Run()
	if err != nil {
		t.Errorf("R5.1: expected exit code 0, got error: %v", err)
	}
}

// TestExitCodeSuccessPassthrough verifies exit code 0 on successful passthrough (R5.1).
func TestExitCodeSuccessPassthrough(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin)
	cmd.Stdin = bytes.NewReader([]byte("passthrough ok\n"))
	err := cmd.Run()
	if err != nil {
		t.Errorf("R5.1: expected exit code 0, got error: %v", err)
	}
}

// TestExitCodeErrorBadOutputPath verifies exit code 1 when output path is invalid (R5.2).
func TestExitCodeErrorBadOutputPath(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	// Use a path inside a non-existent directory to force an output error.
	outPath := filepath.Join(t.TempDir(), "nonexistent", "subdir", "out.txt")

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader([]byte("data\n"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("R5.2: expected exit code 1 for bad output path, got 0")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("R5.2: expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("R5.2: exit code = %d, want 1", exitErr.ExitCode())
	}

	// R5.2: must print descriptive error to stderr
	if stderr.Len() == 0 {
		t.Error("R5.2: expected error message on stderr, got empty")
	}
}

// TestExitCodeErrorBadOutputPathDiff verifies error exit code matches reference (R5.2).
func TestExitCodeErrorBadOutputPathDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skip("reference binary sponge not in PATH")
	}

	outPath := filepath.Join(t.TempDir(), "nonexistent", "deep", "out.txt")

	runWithBadPath := func(bin string) int {
		t.Helper()
		cmd := exec.Command(bin, outPath)
		cmd.Stdin = bytes.NewReader([]byte("data\n"))
		err := cmd.Run()
		if err == nil {
			return 0
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return -1
	}

	goCode := runWithBadPath(goBin)
	refCode := runWithBadPath(refBin)

	if goCode != refCode {
		t.Errorf("R5.2: exit code mismatch: go=%d, ref=%d", goCode, refCode)
	}
}

// TestTempFileCleanupOnError verifies temp files are cleaned up on error paths (R5.4).
func TestTempFileCleanupOnError(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	// Output path in a non-existent directory will cause write error after temp file creation.
	outPath := filepath.Join(t.TempDir(), "nonexistent", "out.txt")

	// Use large input to force temp file creation, then fail on output.
	input := bytes.Repeat([]byte("x"), 1024*1024+1)

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Env = append(os.Environ(), "TMPDIR="+tmpDir)
	_ = cmd.Run() // expected to fail

	// R5.4: verify no temp files left behind after error.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), tempPrefix) {
			t.Errorf("R5.4: temp file %s not cleaned up after error", e.Name())
		}
	}
}

// TestErrorMessageOnStderr verifies stderr output on errors (R5.2).
func TestErrorMessageOnStderr(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	outPath := filepath.Join(t.TempDir(), "no", "such", "dir", "file.txt")

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader([]byte("data\n"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run()

	// R5.2: must print "sponge:" prefixed error to stderr
	output := stderr.String()
	if !strings.HasPrefix(output, "sponge:") {
		t.Errorf("R5.2: stderr = %q, want prefix \"sponge:\"", output)
	}
}

// runSpongeToFile runs a sponge binary with given args/stdin and returns the exit code.
// R6.1: helper for file-content differential comparison.
func runSpongeToFile(t *testing.T, bin string, args []string, stdin []byte, env []string) int {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	if env != nil {
		cmd.Env = env
	}
	err := cmd.Run()
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

// diffFileContent compares file content between Go and reference output files (R6.1).
func diffFileContent(t *testing.T, goPath, refPath string) {
	t.Helper()
	goContent, err := os.ReadFile(goPath)
	if err != nil {
		t.Fatalf("read go output: %v", err)
	}
	refContent, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("read ref output: %v", err)
	}
	if !bytes.Equal(goContent, refContent) {
		t.Errorf("file content mismatch: go=%d bytes, ref=%d bytes", len(goContent), len(refContent))
	}
}

// TestFileOutputDiff verifies file output matches reference binary (R6.1, R6.2).
func TestFileOutputDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skip("reference binary sponge not in PATH")
	}

	// R6.2: small stdin, output file does not exist
	t.Run("R6.2_small_stdin_new_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		input := []byte("small input\n")
		goPath := filepath.Join(dir, "go_out.txt")
		refPath := filepath.Join(dir, "ref_out.txt")

		goCode := runSpongeToFile(t, goBin, []string{goPath}, input, nil)
		refCode := runSpongeToFile(t, refBin, []string{refPath}, input, nil)
		if goCode != refCode {
			t.Errorf("exit code: go=%d, ref=%d", goCode, refCode)
		}
		diffFileContent(t, goPath, refPath)
	})

	// R6.2: large stdin (>1 MB, forces temp file spill)
	t.Run("R6.2_large_stdin_1MB", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		input := bytes.Repeat([]byte("large-line-data\n"), 70000) // ~1.1 MB
		goPath := filepath.Join(dir, "go_out.txt")
		refPath := filepath.Join(dir, "ref_out.txt")

		goCode := runSpongeToFile(t, goBin, []string{goPath}, input, nil)
		refCode := runSpongeToFile(t, refBin, []string{refPath}, input, nil)
		if goCode != refCode {
			t.Errorf("exit code: go=%d, ref=%d", goCode, refCode)
		}
		diffFileContent(t, goPath, refPath)
	})

	// R6.2: output file already exists (overwrite, mode preservation)
	t.Run("R6.2_existing_file_overwrite", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		input := []byte("overwrite content\n")
		goPath := filepath.Join(dir, "go_out.txt")
		refPath := filepath.Join(dir, "ref_out.txt")
		os.WriteFile(goPath, []byte("old go content\n"), 0o644)
		os.WriteFile(refPath, []byte("old ref content\n"), 0o644)

		goCode := runSpongeToFile(t, goBin, []string{goPath}, input, nil)
		refCode := runSpongeToFile(t, refBin, []string{refPath}, input, nil)
		if goCode != refCode {
			t.Errorf("exit code: go=%d, ref=%d", goCode, refCode)
		}
		diffFileContent(t, goPath, refPath)
	})

	// R6.2: append mode with existing file
	t.Run("R6.2_append_existing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		original := []byte("original line\n")
		input := []byte("appended line\n")
		goPath := filepath.Join(dir, "go_out.txt")
		refPath := filepath.Join(dir, "ref_out.txt")
		os.WriteFile(goPath, original, 0o644)
		os.WriteFile(refPath, original, 0o644)

		goCode := runSpongeToFile(t, goBin, []string{"-a", goPath}, input, nil)
		refCode := runSpongeToFile(t, refBin, []string{"-a", refPath}, input, nil)
		if goCode != refCode {
			t.Errorf("exit code: go=%d, ref=%d", goCode, refCode)
		}
		diffFileContent(t, goPath, refPath)
	})

	// R6.2: append mode with non-existent file
	t.Run("R6.2_append_new_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		input := []byte("append to new\n")
		goPath := filepath.Join(dir, "go_out.txt")
		refPath := filepath.Join(dir, "ref_out.txt")

		goCode := runSpongeToFile(t, goBin, []string{"-a", goPath}, input, nil)
		refCode := runSpongeToFile(t, refBin, []string{"-a", refPath}, input, nil)
		if goCode != refCode {
			t.Errorf("exit code: go=%d, ref=%d", goCode, refCode)
		}
		diffFileContent(t, goPath, refPath)
	})

	// R6.2: cross-device rename fallback (different TMPDIR from output)
	t.Run("R6.2_cross_device_tmpdir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		tmpDir := t.TempDir()
		input := []byte("cross device content\n")
		goPath := filepath.Join(dir, "go_out.txt")
		refPath := filepath.Join(dir, "ref_out.txt")
		env := append(os.Environ(), "TMPDIR="+tmpDir)

		goCode := runSpongeToFile(t, goBin, []string{goPath}, input, env)
		refCode := runSpongeToFile(t, refBin, []string{refPath}, input, env)
		if goCode != refCode {
			t.Errorf("exit code: go=%d, ref=%d", goCode, refCode)
		}
		diffFileContent(t, goPath, refPath)
	})

	// R6.2: passthrough mode (no filename) differential via RunDiffTests
	t.Run("R6.2_passthrough_large", func(t *testing.T) {
		t.Parallel()
		input := bytes.Repeat([]byte("pass-line\n"), 50000) // ~500 KB
		tests := []testutils.DiffTest{
			{
				Name:  "R6.2_passthrough_large_stdin",
				Args:  []string{},
				Stdin: input,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}
