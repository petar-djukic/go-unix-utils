// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sponge differential tests verify output parity between the Go sponge
// binary and the reference binary sponge (Homebrew moreutils). Tests compare
// output file contents, stdout, stderr, and exit codes. All tests run with
// LC_ALL=C to eliminate locale-dependent divergence.
//
// Implements: prd007-sponge R1-R4
// Architecture: docs/ARCHITECTURE.yaml (cmd/ component, DD2, DD6)
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinaryName = "sponge"

var (
	goBin  string
	refBin string
)

func TestMain(m *testing.M) {
	ref, err := exec.LookPath(refBinaryName)
	if err == nil {
		refBin = ref
	}

	tmpDir, err := os.MkdirTemp("", "sponge-test-*")
	if err != nil {
		os.Stderr.WriteString("failed to create temp dir: " + err.Error() + "\n")
		os.Exit(1)
	}

	goBin = filepath.Join(tmpDir, "sponge")
	build := exec.Command("go", "build", "-o", goBin, ".")
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		os.RemoveAll(tmpDir) // best-effort cleanup
		os.Stderr.WriteString("go build failed: " + string(out) + "\n")
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir) // best-effort cleanup
	os.Exit(code)
}

// skipIfMissing skips the current test when the reference sponge binary is not
// available on PATH.
func skipIfMissing(t *testing.T) {
	t.Helper()
	if refBin == "" {
		t.Skip(refBinaryName + " not found in PATH")
	}
}

// writeFile creates a file in dir with the given content.
func writeFile(t *testing.T, dir, name string, content []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), content, 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// errPresenceNormalizer replaces any non-empty output with a fixed marker.
// Used for test cases where stderr format differs between implementations but
// both must produce non-empty error output.
func errPresenceNormalizer(b []byte) []byte {
	if len(b) > 0 {
		return []byte("OUTPUT\n")
	}
	return b
}

// genSeqContent generates "1\n2\n...N\n", matching the output of seq 1 N.
func genSeqContent(n int) []byte {
	var buf bytes.Buffer
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&buf, "%d\n", i)
	}
	return buf.Bytes()
}

// testEnv returns an environment slice with LC_ALL=C set, matching the locale
// constraint in ARCHITECTURE DD6.
func testEnv() []string {
	base := os.Environ()
	result := make([]string, 0, len(base)+1)
	for _, kv := range base {
		if !strings.HasPrefix(kv, "LC_ALL=") {
			result = append(result, kv)
		}
	}
	result = append(result, "LC_ALL=C")
	return result
}

// execBinary runs binary with the given args, stdin, env, and working
// directory. Returns stdout, stderr, and exit code. Fails the test on timeout
// or exec failure.
func execBinary(t *testing.T, binary string, args []string, stdin []byte, env []string, dir string) (stdout, stderr []byte, exitCode int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = env
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %q exceeded timeout of 10s", binary)
	}
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			return outBuf.Bytes(), errBuf.Bytes(), exitErr.ExitCode()
		}
		t.Fatalf("exec %q: %v", binary, runErr)
	}
	return outBuf.Bytes(), errBuf.Bytes(), 0
}

// TestSpongeStdinToFile tests basic stdin-to-file writes where sponge reads all
// of stdin before opening the output file.
// (prd007-sponge R1.1, R1.2, R2.1; test-rel01.2: sponge_small_stdin_to_file,
// sponge_empty_stdin)
func TestSpongeStdinToFile(t *testing.T) {
	skipIfMissing(t)

	seqContent := genSeqContent(100)

	tests := []testutils.DiffTest{
		{
			Name:          "small_stdin_to_file",
			Args:          []string{"outfile.txt"},
			Stdin:         seqContent,
			ExitCode:      0,
			ExpectedFiles: map[string][]byte{"outfile.txt": seqContent},
		},
		{
			Name:          "empty_stdin",
			Args:          []string{"empty_out.txt"},
			Stdin:         []byte{},
			ExitCode:      0,
			ExpectedFiles: map[string][]byte{"empty_out.txt": {}},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestSpongeLargeStdin tests that large stdin input (>= 1 MB) is correctly
// written to the output file, matching the reference binary.
// (prd007-sponge R1.3; test-rel01.2: sponge_large_stdin)
func TestSpongeLargeStdin(t *testing.T) {
	skipIfMissing(t)

	line := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789\n")
	count := (1024*1024)/len(line) + 1
	largeContent := bytes.Repeat(line, count)

	tests := []testutils.DiffTest{
		{
			Name:          "large_stdin",
			Args:          []string{"large_out.txt"},
			Stdin:         largeContent,
			ExitCode:      0,
			ExpectedFiles: map[string][]byte{"large_out.txt": largeContent},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestSpongeSoakBeforeWrite tests the soak-before-write contract by writing to
// a pre-existing file. The stdin content matches the initial file content,
// verifying that the overwrite produces the correct result and sponge does not
// corrupt the file.
// (prd007-sponge R1.1, R2.5; test-rel01.2: sponge_soak_before_write)
func TestSpongeSoakBeforeWrite(t *testing.T) {
	skipIfMissing(t)

	content := []byte("line1\nline2\nline3\n")
	dir := t.TempDir()
	writeFile(t, dir, "data.txt", content)

	tests := []testutils.DiffTest{
		{
			Name:          "soak_before_write",
			Args:          []string{"data.txt"},
			Stdin:         content,
			WorkDir:       dir,
			ExitCode:      0,
			ExpectedFiles: map[string][]byte{"data.txt": content},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestSpongeAppendMode tests the -a flag. The append_to_existing subtest uses
// separate working directories for each binary because the reference binary's
// append modifies the file state that the Go binary would otherwise inherit.
// (prd007-sponge R3.1, R3.2, R3.3; test-rel01.2: sponge_append_mode,
// sponge_append_mode_no_existing_file)
func TestSpongeAppendMode(t *testing.T) {
	skipIfMissing(t)

	t.Run("append_to_existing", func(t *testing.T) {
		initial := []byte("original line\n")
		stdin := []byte("appended line\n")
		args := []string{"-a", "existing.txt"}

		refDir := t.TempDir()
		goDir := t.TempDir()
		writeFile(t, refDir, "existing.txt", initial)
		writeFile(t, goDir, "existing.txt", initial)

		env := testEnv()
		refStdout, refStderr, refCode := execBinary(t, refBin, args, stdin, env, refDir)
		goStdout, goStderr, goCode := execBinary(t, goBin, args, stdin, env, goDir)

		if !bytes.Equal(refStdout, goStdout) || !bytes.Equal(refStderr, goStderr) || refCode != goCode {
			t.Fatalf("output divergence:\n"+
				"  args:       %v\n"+
				"  ref stdout: %q\n"+
				"   go stdout: %q\n"+
				"  ref stderr: %q\n"+
				"   go stderr: %q\n"+
				"  ref exit:   %d\n"+
				"   go exit:   %d\n",
				args, refStdout, goStdout, refStderr, goStderr, refCode, goCode)
		}

		refContent, err := os.ReadFile(filepath.Join(refDir, "existing.txt"))
		if err != nil {
			t.Fatalf("read ref output file: %v", err)
		}
		goContent, err := os.ReadFile(filepath.Join(goDir, "existing.txt"))
		if err != nil {
			t.Fatalf("read go output file: %v", err)
		}
		if !bytes.Equal(refContent, goContent) {
			t.Fatalf("file content divergence:\n  ref: %q\n   go: %q", refContent, goContent)
		}
	})

	t.Run("append_no_existing_file", func(t *testing.T) {
		stdin := []byte("new content\n")
		tests := []testutils.DiffTest{
			{
				Name:          "creates_new_file",
				Args:          []string{"-a", "newfile.txt"},
				Stdin:         stdin,
				ExitCode:      0,
				ExpectedFiles: map[string][]byte{"newfile.txt": stdin},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// TestSpongeErrors tests error handling for invalid arguments and write
// failures. Error message format differs between implementations, so stderr is
// normalized to a presence check.
// (prd007-sponge R5.1, R5.2)
func TestSpongeErrors(t *testing.T) {
	skipIfMissing(t)

	tests := []testutils.DiffTest{
		{
			Name:      "invalid_flag",
			Args:      []string{"-z", "out.txt"},
			Stdin:     []byte{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errPresenceNormalizer},
		},
		{
			Name:      "write_to_nonexistent_dir",
			Args:      []string{"nonexistent_dir/output.txt"},
			Stdin:     []byte("test\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errPresenceNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
