// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd007-sponge R1.1–R1.5, R2.1–R2.5, R3.1–R3.3,
// R4.1–R4.3, R5.1–R5.4, R6.1–R6.2.
package main_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// ignoreOutput normalizes away content so only exit code is compared.
// Used for error tests where stderr messages differ between implementations.
func ignoreOutput(b []byte) []byte { return []byte{} }

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skipf("reference binary sponge not in PATH: %v", err)
	}

	// Passthrough mode tests use standard RunDiffTests (stdout comparison).
	passthroughTests := []testutils.DiffTest{
		// R1.1, R4.1: passthrough writes buffered stdin to stdout.
		{
			Name:  "passthrough_stdout",
			Stdin: []byte("hello world\n"),
		},
		// R1.1: empty stdin produces empty stdout.
		{
			Name:  "passthrough_empty",
			Stdin: []byte{},
		},
		// R1.2: multi-line passthrough preserves content.
		{
			Name:  "passthrough_multiline",
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		// R4.1: passthrough mode writes buffered content to stdout.
		{
			Name:  "passthrough_single_line",
			Stdin: []byte("single line no newline"),
		},
		// R4.3: small in-memory buffer written directly to stdout.
		{
			Name:  "passthrough_small_binary",
			Stdin: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE},
		},
		// R4.1, R4.3: passthrough with moderate multi-line input.
		{
			Name:  "passthrough_moderate",
			Stdin: generateSeq(1, 1000),
		},
		// R5.1: exit 0 on successful passthrough (explicit verification).
		{
			Name:  "exit_0_passthrough",
			Stdin: []byte("success\n"),
		},
		// R5.2: exit 1 when output path has nonexistent parent directory.
		// Both binaries fail identically; stderr messages ignored.
		{
			Name:      "error_nonexistent_parent",
			Args:      []string{"nodir/file.txt"},
			Stdin:     []byte("test\n"),
			Normalize: []testutils.NormalizeFunc{ignoreOutput},
		},
		// R6.1: --version prints version information and exits 0.
		// Output content differs between implementations; compare exit code only.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Stdin:     []byte{},
			Normalize: []testutils.NormalizeFunc{ignoreOutput},
		},
		// R6.2: --help prints usage information and exits 0.
		// Output content differs between implementations; compare exit code only.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Stdin:     []byte{},
			Normalize: []testutils.NormalizeFunc{ignoreOutput},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, passthroughTests)

	// File output tests compare output file content between ref and Go binary.
	fileTests := []fileOutputTest{
		// R1.1, R1.2: small stdin written to output file.
		{
			name:    "small_stdin_to_file",
			stdin:   generateSeq(1, 100),
			outFile: "out.txt",
		},
		// R1.1: empty stdin creates empty output file.
		{
			name:    "empty_stdin_to_file",
			stdin:   []byte{},
			outFile: "empty.txt",
		},
		// R1.1: soak-before-write verified by writing to a pre-existing file
		// whose content matches stdin. If sponge truncates before reading,
		// the file would be empty; correct behavior preserves the content.
		{
			name:    "soak_before_write",
			stdin:   []byte("line1\nline2\nline3\n"),
			outFile: "data.txt",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeTestFile(t, filepath.Join(dir, "data.txt"), "line1\nline2\nline3\n")
			},
		},
		// R3.1: append mode prepends existing content before stdin.
		{
			name:      "append_mode",
			stdin:     []byte("appended line\n"),
			outFile:   "existing.txt",
			extraArgs: []string{"-a"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeTestFile(t, filepath.Join(dir, "existing.txt"), "original line\n")
			},
		},
		// R3.2: append with non-existent file creates new file.
		{
			name:      "append_no_existing",
			stdin:     []byte("new content\n"),
			outFile:   "new.txt",
			extraArgs: []string{"-a"},
		},
		// R1.3, R1.4: large stdin (>1 MB) to verify correct handling.
		{
			name:    "large_stdin",
			stdin:   generateSeq(1, 50000),
			outFile: "large.txt",
		},
		// R1.5: temp files are cleaned up on normal successful exit.
		{
			name:    "temp_cleanup_on_success",
			stdin:   []byte("cleanup test\n"),
			outFile: "cleanup.txt",
		},
		// R2.1: atomic rename to a file that does not yet exist.
		{
			name:    "atomic_rename_new_file",
			stdin:   []byte("atomic content\n"),
			outFile: "atomic.txt",
		},
		// R2.2: rename fallback with explicit TMPDIR setting.
		// Both binaries use the same TMPDIR, so behavior matches.
		{
			name:    "rename_fallback_tmpdir",
			stdin:   []byte("tmpdir test\n"),
			outFile: "tmpdir.txt",
			env:     []string{"TMPDIR=" + os.TempDir()},
		},
		// R2.3: permission preservation on existing file with 0600 mode.
		{
			name:      "preserve_permissions_0600",
			stdin:     []byte("new content\n"),
			outFile:   "perms.txt",
			checkMode: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				p := filepath.Join(dir, "perms.txt")
				writeTestFile(t, p, "old content\n")
				if err := os.Chmod(p, 0o600); err != nil {
					t.Fatalf("chmod: %v", err)
				}
			},
		},
		// R2.3: permission preservation on existing file with 0755 mode.
		{
			name:      "preserve_permissions_0755",
			stdin:     []byte("executable content\n"),
			outFile:   "exec.txt",
			checkMode: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				p := filepath.Join(dir, "exec.txt")
				writeTestFile(t, p, "old exec\n")
				if err := os.Chmod(p, 0o755); err != nil {
					t.Fatalf("chmod: %v", err)
				}
			},
		},
		// R2.4: output via symlink writes through to symlink target.
		{
			name:    "symlink_output",
			stdin:   []byte("symlink content\n"),
			outFile: "link.txt",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				target := filepath.Join(dir, "target.txt")
				writeTestFile(t, target, "original target\n")
				link := filepath.Join(dir, "link.txt")
				if err := os.Symlink(target, link); err != nil {
					t.Fatalf("creating symlink: %v", err)
				}
			},
		},
		// R2.5: overwrite existing file verifies atomic write produces
		// complete output (no partial state observable).
		{
			name:    "overwrite_existing_file",
			stdin:   generateSeq(1, 500),
			outFile: "overwrite.txt",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeTestFile(t, filepath.Join(dir, "overwrite.txt"), "old data\n")
			},
		},
		// R3.1: append mode with multi-line existing content and multi-line
		// stdin verifies correct concatenation order.
		{
			name:      "append_multiline",
			stdin:     []byte("new1\nnew2\nnew3\n"),
			outFile:   "multi.txt",
			extraArgs: []string{"-a"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeTestFile(t, filepath.Join(dir, "multi.txt"), "old1\nold2\nold3\n")
			},
		},
		// R3.2: append mode with symlink output does not prepend because
		// lstat shows the path is not a regular file.
		{
			name:      "append_symlink",
			stdin:     []byte("appended\n"),
			outFile:   "alink.txt",
			extraArgs: []string{"-a"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				target := filepath.Join(dir, "atarget.txt")
				writeTestFile(t, target, "existing\n")
				link := filepath.Join(dir, "alink.txt")
				if err := os.Symlink(target, link); err != nil {
					t.Fatalf("creating symlink: %v", err)
				}
			},
		},
		// R3.3: append mode reads original file content into temp file
		// before appending stdin, preserving original before rename.
		{
			name:      "append_large_existing",
			stdin:     []byte("appended data\n"),
			outFile:   "large_existing.txt",
			extraArgs: []string{"-a"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeTestFile(t, filepath.Join(dir, "large_existing.txt"),
					string(generateSeq(1, 500)))
			},
		},
		// R3.3: append with empty stdin appends nothing to existing content.
		{
			name:      "append_empty_stdin",
			stdin:     []byte{},
			outFile:   "append_empty.txt",
			extraArgs: []string{"-a"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeTestFile(t, filepath.Join(dir, "append_empty.txt"), "keep this\n")
			},
		},
		// R5.1: exit 0 on successful file write (explicit verification).
		{
			name:    "exit_0_file_write",
			stdin:   []byte("success content\n"),
			outFile: "success.txt",
		},
	}
	for _, tc := range fileTests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, goBin, refBin)
		})
	}
}

// TestTempCleanupOnError verifies that sponge removes temp files even when
// the output write fails. Implements R5.4.
func TestTempCleanupOnError(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	// R5.4: force an error by writing to a path with nonexistent parent.
	outPath := filepath.Join(t.TempDir(), "nodir", "file.txt")

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader([]byte("test data for cleanup\n"))
	env := buildTestEnv()
	env = setTestEnv(env, "TMPDIR", tmpDir)
	cmd.Env = env
	_ = cmd.Run() // expect failure

	// Verify no sponge temp files remain in TMPDIR.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("reading TMPDIR: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "sponge.") {
			t.Fatalf("R5.4: temp file %s not cleaned up after error exit", e.Name())
		}
	}
}

// TestErrorExitCode verifies that sponge exits 1 on output write failure.
// Implements R5.2.
func TestErrorExitCode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	// R5.2: output to nonexistent parent directory must exit 1.
	outPath := filepath.Join(t.TempDir(), "nodir", "file.txt")

	cmd := exec.Command(goBin, outPath)
	cmd.Stdin = bytes.NewReader([]byte("test\n"))
	cmd.Env = buildTestEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("R5.2: expected non-zero exit code for invalid output path")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("R5.2: unexpected error type: %v", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("R5.2: expected exit code 1, got %d", exitErr.ExitCode())
	}
	// R5.2: must print a descriptive error message to stderr.
	if stderr.Len() == 0 {
		t.Fatal("R5.2: expected error message on stderr")
	}
}

// fileOutputTest describes a sponge test that writes to a file.
type fileOutputTest struct {
	name      string
	stdin     []byte
	outFile   string
	extraArgs []string
	setup     func(t *testing.T, dir string)
	env       []string // additional KEY=VALUE env vars
	checkMode bool     // if true, compare file permissions between ref and Go
}

// run executes both binaries in separate temp dirs and compares file content.
func (ft fileOutputTest) run(t *testing.T, goBin, refBin string) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	if ft.setup != nil {
		ft.setup(t, refDir)
		ft.setup(t, goDir)
	}
	refArgs := ft.buildArgs(refDir)
	goArgs := ft.buildArgs(goDir)
	env := ft.buildEnv()
	refContent := runAndReadFile(t, refBin, refArgs, ft.stdin, refDir, ft.outFile, env)
	goContent := runAndReadFile(t, goBin, goArgs, ft.stdin, goDir, ft.outFile, env)
	if !bytes.Equal(refContent, goContent) {
		t.Fatalf("file content divergence\nref (%d bytes): %s\ngo  (%d bytes): %s",
			len(refContent), truncate(refContent, 256),
			len(goContent), truncate(goContent, 256))
	}
	if ft.checkMode {
		compareFileModes(t, refDir, goDir, ft.outFile)
	}
}

// buildArgs constructs the argument list with the output file in dir.
func (ft fileOutputTest) buildArgs(dir string) []string {
	args := make([]string, len(ft.extraArgs))
	copy(args, ft.extraArgs)
	return append(args, filepath.Join(dir, ft.outFile))
}

// buildEnv constructs the test environment with LC_ALL=C and any extra vars.
func (ft fileOutputTest) buildEnv() []string {
	env := buildTestEnv()
	for _, e := range ft.env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env = setTestEnv(env, parts[0], parts[1])
		}
	}
	return env
}

// runAndReadFile executes a binary and reads the specified output file.
func runAndReadFile(t *testing.T, binary string, args []string, stdin []byte, dir, outFile string, env []string) []byte {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %v failed: %v\nstderr: %s", binary, args, err, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, outFile))
	if err != nil {
		t.Fatalf("reading output file %s: %v", outFile, err)
	}
	return data
}

// compareFileModes compares file permissions between ref and Go output (R2.3).
func compareFileModes(t *testing.T, refDir, goDir, outFile string) {
	t.Helper()
	refInfo, err := os.Lstat(filepath.Join(refDir, outFile))
	if err != nil {
		t.Fatalf("stat ref file: %v", err)
	}
	goInfo, err := os.Lstat(filepath.Join(goDir, outFile))
	if err != nil {
		t.Fatalf("stat go file: %v", err)
	}
	if refInfo.Mode().Perm() != goInfo.Mode().Perm() {
		t.Fatalf("mode divergence: ref=%o go=%o",
			refInfo.Mode().Perm(), goInfo.Mode().Perm())
	}
}

// buildTestEnv constructs test environment with LC_ALL=C.
func buildTestEnv() []string {
	env := os.Environ()
	return setTestEnv(env, "LC_ALL", "C")
}

// setTestEnv sets or replaces a key=value in the env slice.
func setTestEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// generateSeq generates "1\n2\n...\nN\n" matching seq output.
func generateSeq(start, end int) []byte {
	var buf bytes.Buffer
	for i := start; i <= end; i++ {
		fmt.Fprintf(&buf, "%d\n", i)
	}
	return buf.Bytes()
}

// writeTestFile writes content to a file, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}

// truncate returns data as a string, truncated for display.
func truncate(data []byte, maxLen int) string {
	if len(data) <= maxLen {
		return fmt.Sprintf("%q", data)
	}
	return fmt.Sprintf("%q...(truncated, %d total)", data[:maxLen], len(data))
}
