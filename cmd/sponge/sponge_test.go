// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sponge comparing Go binary against moreutils
// reference sponge binary.
//
// Implements: prd007-sponge R1, R2, R3, R4, R5
// Traces: test-rel01.2, rel01.2-uc001-sponge
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

// goBinaryPath is the path to the compiled Go sponge binary.
var goBinaryPath string

// refBinaryPath is the path to the moreutils reference binary (sponge).
var refBinaryPath string

func TestMain(m *testing.M) {
	// Locate moreutils reference binary (D1).
	ref, err := exec.LookPath("sponge")
	if err != nil {
		fmt.Println("sponge not found; skipping sponge differential tests")
		os.Exit(0)
	}
	refBinaryPath = ref

	// Build the Go sponge binary to a temp directory.
	tmpDir, err := os.MkdirTemp("", "sponge-test-*")
	if err != nil {
		panic("creating temp dir: " + err.Error())
	}

	goBinaryPath = filepath.Join(tmpDir, "sponge")
	cmd := exec.Command("go", "build", "-o", goBinaryPath, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tmpDir)
		panic(fmt.Sprintf("building sponge binary: %v\n%s", err, out))
	}

	code := m.Run()
	os.RemoveAll(tmpDir) // best-effort cleanup
	os.Exit(code)
}

// --- File-output differential test infrastructure (D3) ---

// fileDiffTest defines a differential test case for file-output utilities
// where both binaries write to files rather than stdout.
type fileDiffTest struct {
	Name            string               // subtest name
	Args            []string             // command-line arguments
	Stdin           []byte               // stdin content
	PreFiles        map[string]fileSetup // files to create before running each binary
	CheckFiles      []string             // filenames to compare after running
	SkipStderrMatch bool                 // skip exact stderr comparison for implementation-specific error messages
}

// fileSetup describes a pre-existing file to create before running a binary.
type fileSetup struct {
	Content []byte
	Mode    os.FileMode
}

// binaryResult holds captured output from a single binary invocation.
type binaryResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// fileDiffTimeout is the maximum duration each binary invocation may run.
const fileDiffTimeout = 10 * time.Second

// runFileDiffTests runs each fileDiffTest as a named subtest.
func runFileDiffTests(t *testing.T, tests []fileDiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleFileDiffTest(t, tc)
		})
	}
}

// runSingleFileDiffTest executes both binaries in separate directories with
// identical pre-conditions and compares output files, stderr, and exit code.
func runSingleFileDiffTest(t *testing.T, tc fileDiffTest) {
	t.Helper()

	refDir := t.TempDir()
	goDir := t.TempDir()

	// Set up identical pre-existing files in both directories.
	for name, setup := range tc.PreFiles {
		for _, dir := range []string{refDir, goDir} {
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, setup.Content, setup.Mode); err != nil {
				t.Fatalf("writing pre-file %s: %v", name, err)
			}
		}
	}

	env := buildTestEnv()

	refResult := execBinary(t, refBinaryPath, tc.Args, tc.Stdin, env, refDir)
	goResult := execBinary(t, goBinaryPath, tc.Args, tc.Stdin, env, goDir)

	failed := false

	// Compare exit codes.
	if refResult.ExitCode != goResult.ExitCode {
		failed = true
	}

	// Compare stderr.
	if !tc.SkipStderrMatch {
		if !bytes.Equal(refResult.Stderr, goResult.Stderr) {
			failed = true
		}
	} else {
		// For error tests, verify both produce stderr or both produce none.
		if (len(refResult.Stderr) > 0) != (len(goResult.Stderr) > 0) {
			failed = true
		}
	}

	// Compare output files byte-for-byte.
	for _, name := range tc.CheckFiles {
		refPath := filepath.Join(refDir, name)
		goPath := filepath.Join(goDir, name)

		refContent, refErr := os.ReadFile(refPath)
		goContent, goErr := os.ReadFile(goPath)

		if refErr != nil && goErr != nil {
			continue // both failed to create the file — consistent
		}
		if (refErr != nil) != (goErr != nil) {
			failed = true
			t.Errorf("file %s existence divergence: ref err=%v, go err=%v", name, refErr, goErr)
			continue
		}
		if !bytes.Equal(refContent, goContent) {
			failed = true
			t.Errorf("file content divergence for %s\nref: %q\ngo:  %q", name, refContent, goContent)
		}
	}

	if failed {
		t.Errorf("divergence detected\n"+
			"args:       %v\n"+
			"ref stderr: %q\n"+
			"go  stderr: %q\n"+
			"ref exit:   %d\n"+
			"go  exit:   %d",
			tc.Args,
			refResult.Stderr,
			goResult.Stderr,
			refResult.ExitCode,
			goResult.ExitCode,
		)
	}
}

// buildTestEnv constructs the environment for binary invocations with LC_ALL=C.
func buildTestEnv() []string {
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "LC_ALL=") {
			env[i] = "LC_ALL=C"
			return env
		}
	}
	return append(env, "LC_ALL=C")
}

// execBinary runs a binary with the given arguments, stdin, environment, and
// working directory, capturing stdout, stderr, and exit code.
func execBinary(t *testing.T, binPath string, args []string, stdin []byte, env []string, workDir string) binaryResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), fileDiffTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Env = env
	cmd.Dir = workDir

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("binary %s timed out after %v", binPath, fileDiffTimeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("executing %s: %v", binPath, err)
		}
	}

	return binaryResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: exitCode,
	}
}

// --- Passthrough mode: no output filename (prd007-sponge R4) ---

func TestPassthrough_Stdin(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "passthrough-text",
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "passthrough-multiline",
			Stdin: []byte("line one\nline two\nline three\n"),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestPassthrough_EmptyStdin(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "passthrough-empty",
			Stdin: []byte(""),
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- File output mode (prd007-sponge R1, R2) ---

func TestFile_SmallStdin(t *testing.T) {
	var buf bytes.Buffer
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&buf, "%d\n", i)
	}

	runFileDiffTests(t, []fileDiffTest{
		{
			Name:       "small-stdin-to-file",
			Args:       []string{"outfile.txt"},
			Stdin:      buf.Bytes(),
			CheckFiles: []string{"outfile.txt"},
		},
	})
}

func TestFile_EmptyStdin(t *testing.T) {
	runFileDiffTests(t, []fileDiffTest{
		{
			Name:       "empty-stdin-to-file",
			Args:       []string{"empty_out.txt"},
			Stdin:      []byte(""),
			CheckFiles: []string{"empty_out.txt"},
		},
	})
}

func TestFile_SoakBeforeWrite(t *testing.T) {
	content := []byte("line1\nline2\nline3\n")

	runFileDiffTests(t, []fileDiffTest{
		{
			Name:  "soak-before-write",
			Args:  []string{"data.txt"},
			Stdin: content,
			PreFiles: map[string]fileSetup{
				"data.txt": {Content: content, Mode: 0o644},
			},
			CheckFiles: []string{"data.txt"},
		},
	})
}

// --- Append mode (prd007-sponge R3) ---

func TestAppend_ExistingFile(t *testing.T) {
	runFileDiffTests(t, []fileDiffTest{
		{
			Name:  "append-existing-file",
			Args:  []string{"-a", "existing.txt"},
			Stdin: []byte("appended line\n"),
			PreFiles: map[string]fileSetup{
				"existing.txt": {Content: []byte("original line\n"), Mode: 0o644},
			},
			CheckFiles: []string{"existing.txt"},
		},
	})
}

func TestAppend_NonexistentFile(t *testing.T) {
	runFileDiffTests(t, []fileDiffTest{
		{
			Name:       "append-nonexistent-file",
			Args:       []string{"-a", "newfile.txt"},
			Stdin:      []byte("new content\n"),
			CheckFiles: []string{"newfile.txt"},
		},
	})
}

// --- Error handling (prd007-sponge R5) ---

func TestError_NonexistentDirectory(t *testing.T) {
	runFileDiffTests(t, []fileDiffTest{
		{
			Name:            "error-nonexistent-dir",
			Args:            []string{filepath.Join("nonexistent", "dir", "file.txt")},
			Stdin:           []byte("test\n"),
			SkipStderrMatch: true,
		},
	})
}
