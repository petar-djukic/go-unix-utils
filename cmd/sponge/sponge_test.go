// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests: prd007-sponge R3.3, R4.1, R4.2, R4.3, R5.1, R5.2, R5.3, R5.4, R6.1, R6.2
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiffPassthrough exercises passthrough mode (no output filename) against
// the gsponge reference binary, covering R4.1 (no filename → stdout), R4.2
// (large input that would trigger temp file spill), and R4.3 (small in-memory
// input written directly to stdout).
func TestDiffPassthrough(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsponge")
	if err != nil {
		t.Skipf("reference binary gsponge not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R4.1, R4.3: passthrough mode with small (in-memory) stdin.
			Name:  "passthrough_small",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1, R4.2: passthrough mode with large stdin (1 MB+, forces temp file spill path).
			Name:  "passthrough_large",
			Args:  []string{},
			Stdin: bytes.Repeat([]byte("x"), 1024*1024+1),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1, R4.3: passthrough mode with empty stdin produces empty stdout.
			Name:  "passthrough_empty",
			Args:  []string{},
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.1, R4.3: passthrough mode with multi-line stdin.
			Name:  "passthrough_multiline",
			Args:  []string{},
			Stdin: []byte("line1\nline2\nline3\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffAppendMode exercises append mode (-a flag) against the gsponge
// reference binary, verifying R3.3: the prepend operation copies the original
// file content into the temp file first, then appends the stdin buffer.
func TestDiffAppendMode(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsponge")
	if err != nil {
		t.Skipf("reference binary gsponge not in PATH: %v", err)
	}

	cases := []struct {
		name     string
		original []byte
		stdin    []byte
	}{
		{
			// R3.3: original file content prepended before stdin content.
			name:     "append_prepends_original_content",
			original: []byte("original content\n"),
			stdin:    []byte("stdin content\n"),
		},
		{
			// R3.3: multi-line original file content fully preserved before stdin.
			name:     "append_multiline_original",
			original: []byte("line1\nline2\nline3\n"),
			stdin:    []byte("appended\n"),
		},
		{
			// R3.3: append to file with binary content is handled correctly.
			name:     "append_binary_original",
			original: []byte{0x01, 0x02, 0x03},
			stdin:    []byte{0x04, 0x05},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refContent := runSpongeAppend(t, refBin, tc.original, tc.stdin)
			goContent := runSpongeAppend(t, goBin, tc.original, tc.stdin)
			if !bytes.Equal(refContent, goContent) {
				t.Errorf("file content mismatch for %q:\nref: %q\ngot: %q", tc.name, refContent, goContent)
			}
		})
	}
}

// runSpongeAppend writes originalContent to a temp directory, runs binary with
// -a against that file, and returns the resulting file contents. Used for
// differential testing of append mode (R3.3).
func runSpongeAppend(t *testing.T, binary string, originalContent, stdin []byte) []byte {
	t.Helper()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(outFile, originalContent, 0o644); err != nil {
		t.Fatalf("writing original file: %v", err)
	}
	cmd := exec.Command(binary, "-a", "out.txt")
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running %q: %v\noutput: %s", binary, err, out)
	}
	result, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	return result
}

// TestDiff runs the full differential test suite for sponge against gsponge,
// covering the four core scenarios defined in prd007-sponge R5.1–R5.4. Each
// file-write case includes ExpectedFiles to verify output file content via
// pkg/testutils (R6.1).
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsponge")
	if err != nil {
		t.Skipf("reference binary gsponge not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R5.1, R6.1: basic stdin-to-file write -- verify all buffered stdin
			// bytes are written to the named output file; ExpectedFiles checks
			// file content via pkg/testutils (R6.1).
			Name:    "basic_file_write",
			Args:    []string{"output.txt"},
			Stdin:   []byte("hello, sponge\n"),
			Env:     []string{"LC_ALL=C"},
			WorkDir: t.TempDir(),
			ExpectedFiles: map[string][]byte{
				"output.txt": []byte("hello, sponge\n"),
			},
		},
		{
			// R5.2, R6.1: append mode (-a flag) creates the file when absent;
			// ExpectedFiles verifies the file content (R6.1).
			Name:    "append_mode",
			Args:    []string{"-a", "output.txt"},
			Stdin:   []byte("appended line\n"),
			Env:     []string{"LC_ALL=C"},
			WorkDir: t.TempDir(),
			ExpectedFiles: map[string][]byte{
				"output.txt": []byte("appended line\n"),
			},
		},
		{
			// R5.3: stdout mode (no file argument) -- verify stdin bytes are
			// written to stdout when no output file is specified.
			Name:  "stdout_mode",
			Stdin: []byte("output to stdout\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R5.4, R6.1: empty stdin, file mode -- verify zero-byte input
			// produces an empty output file; ExpectedFiles verifies file content
			// (R6.1).
			Name:    "empty_stdin_file",
			Args:    []string{"output.txt"},
			Stdin:   []byte{},
			Env:     []string{"LC_ALL=C"},
			WorkDir: t.TempDir(),
			ExpectedFiles: map[string][]byte{
				"output.txt": {},
			},
		},
		{
			// R5.4: empty stdin, stdout mode -- verify zero-byte input produces
			// empty stdout and exit 0 when no file argument is given.
			Name:  "empty_stdin_stdout",
			Stdin: []byte{},
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runSpongeFile creates an isolated temp directory, optionally writes
// initialContent to "out.txt", runs binary with stdin and "out.txt" as the
// file argument, then returns the resulting file content.
//
// R6.1: Used for differential file-content comparison between the Go binary
// and the reference binary in file-write mode.
func runSpongeFile(t *testing.T, binary string, initialContent, stdin []byte, extraEnv []string) []byte {
	t.Helper()
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")
	if initialContent != nil {
		if err := os.WriteFile(outFile, initialContent, 0o644); err != nil {
			t.Fatalf("writing initial file: %v", err)
		}
	}
	cmd := exec.Command(binary, "out.txt")
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Env = append(cmd.Env, extraEnv...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running %q: %v\noutput: %s", binary, err, out)
	}
	result, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	return result
}

// TestDiffFileContent verifies that the Go binary produces identical output
// file content to the reference binary for file-write mode. Covers R6.1
// (differential file-content comparison) and R6.2 (small stdin, large stdin
// that forces temp file spill, and overwriting an existing file).
func TestDiffFileContent(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsponge")
	if err != nil {
		t.Skipf("reference binary gsponge not in PATH: %v", err)
	}

	cases := []struct {
		name    string
		stdin   []byte
		initial []byte // nil means output file does not exist before sponge runs
	}{
		{
			// R6.2: small stdin (fits in memory), output file does not exist.
			name:  "small_stdin_new_file",
			stdin: []byte("hello, sponge\n"),
		},
		{
			// R6.2: large stdin (>1 MB, forces temp file spill in reference binary).
			name:  "large_stdin_new_file",
			stdin: bytes.Repeat([]byte("z"), 1024*1024+1),
		},
		{
			// R6.2: output file already exists — sponge overwrites it.
			name:    "overwrite_existing_file",
			stdin:   []byte("new content\n"),
			initial: []byte("old content\n"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refContent := runSpongeFile(t, refBin, tc.initial, tc.stdin, nil)
			goContent := runSpongeFile(t, goBin, tc.initial, tc.stdin, nil)
			if !bytes.Equal(refContent, goContent) {
				t.Errorf("file content mismatch:\nref: %q\ngot: %q", refContent, goContent)
			}
		})
	}
}

// TestFileModePreservation verifies that sponge preserves the file mode
// (permissions) of an existing output file (prd007-sponge R2.3, R6.2: output
// file already exists, mode preservation). Compares the Go binary's behavior
// against the reference binary.
func TestFileModePreservation(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsponge")
	if err != nil {
		t.Skipf("reference binary gsponge not in PATH: %v", err)
	}

	const initialMode = os.FileMode(0o755)
	stdin := []byte("replacement content\n")

	// getMode creates a file with initialMode, runs binary against it, and
	// returns the resulting file permissions.
	getMode := func(t *testing.T, binary string) os.FileMode {
		t.Helper()
		dir := t.TempDir()
		outFile := filepath.Join(dir, "out.txt")
		if err := os.WriteFile(outFile, []byte("original\n"), initialMode); err != nil {
			t.Fatalf("writing initial file: %v", err)
		}
		cmd := exec.Command(binary, "out.txt")
		cmd.Stdin = bytes.NewReader(stdin)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("running %q: %v\noutput: %s", binary, err, out)
		}
		info, err := os.Stat(outFile)
		if err != nil {
			t.Fatalf("stat output file: %v", err)
		}
		return info.Mode().Perm()
	}

	refMode := getMode(t, refBin)
	goMode := getMode(t, goBin)

	if refMode != goMode {
		t.Errorf("file mode mismatch: ref=%04o got=%04o", refMode, goMode)
	}
}

// TestCrossDeviceRename verifies the cross-device rename fallback (prd007-sponge
// R2.2, R6.2) by placing the output file on a different filesystem from TMPDIR.
// The test skips when both directories are on the same device, which is the
// typical case on a single-partition development machine.
func TestCrossDeviceRename(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsponge")
	if err != nil {
		t.Skipf("reference binary gsponge not in PATH: %v", err)
	}

	tmpDir := t.TempDir()
	outDir := t.TempDir()

	var statTmp, statOut syscall.Stat_t
	if err := syscall.Stat(tmpDir, &statTmp); err != nil {
		t.Skipf("stat %s: %v", tmpDir, err)
	}
	if err := syscall.Stat(outDir, &statOut); err != nil {
		t.Skipf("stat %s: %v", outDir, err)
	}
	if statTmp.Dev == statOut.Dev {
		t.Skip("tmpDir and outDir are on the same device; cross-device rename fallback requires separate filesystems")
	}

	stdin := []byte("cross-device content\n")

	// runCrossDevice runs binary with the given output file path and TMPDIR set
	// to tmpDir (a different device), then returns the resulting file content.
	runCrossDevice := func(t *testing.T, binary, outFile string) []byte {
		t.Helper()
		cmd := exec.Command(binary, outFile)
		cmd.Stdin = bytes.NewReader(stdin)
		cmd.Env = append(os.Environ(), "LC_ALL=C", "TMPDIR="+tmpDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("running %q: %v\noutput: %s", binary, err, out)
		}
		result, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatalf("reading output file: %v", err)
		}
		return result
	}

	refContent := runCrossDevice(t, refBin, filepath.Join(outDir, "ref_out.txt"))
	goContent := runCrossDevice(t, goBin, filepath.Join(outDir, "go_out.txt"))

	if !bytes.Equal(refContent, goContent) {
		t.Errorf("cross-device file content mismatch:\nref: %q\ngot: %q", refContent, goContent)
	}
}
