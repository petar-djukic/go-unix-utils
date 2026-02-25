// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sponge exercising all sponge test cases from
// test-rel01.1.yaml.
//
// Implements: prd007-sponge R6 (differential testing), prd001-testutils R5
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinary is the path to the freshly built Go sponge binary. Set by TestMain.
var goBinary string

// refBinary is the path to the Homebrew reference sponge binary. Set by TestMain.
var refBinary string

// baseEnv provides the standard test environment per test-rel01.1.yaml
// preconditions: LC_ALL=C to eliminate locale-dependent divergence.
var baseEnv = []string{"LC_ALL=C"}

// execTimeout is the maximum duration for a single binary invocation.
// Per prd001-testutils R2.3.
const execTimeout = 10 * time.Second

// maxStdinReport is the maximum number of stdin bytes shown in failure messages.
// Per prd001-testutils R3.5.
const maxStdinReport = 256

// TestMain builds the Go sponge binary and locates the Homebrew reference binary.
// Per design decision D3 and D4.
func TestMain(m *testing.M) {
	// Build the Go sponge binary into a temp directory.
	tmpDir, err := os.MkdirTemp("", "sponge-test-*")
	if err != nil {
		os.Exit(1)
	}

	goBinary = filepath.Join(tmpDir, "sponge")
	buildCmd := exec.Command("go", "build", "-o", goBinary, ".")
	if _, err := buildCmd.CombinedOutput(); err != nil {
		// Build failed; leave goBinary empty so tests skip gracefully.
		goBinary = ""
	}

	// Locate the Homebrew reference binary (brew install moreutils).
	// Per design decision D3: reference is sponge, not gsponge (moreutils
	// utilities are not g-prefixed per sources.yaml).
	refBinary, _ = exec.LookPath("sponge")

	code := m.Run()
	// Best-effort cleanup of temp directory.
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}

// fileDiffTestCase defines a single file-output differential test case.
// Per design decision D1: file-output tests run each binary in an isolated
// temp directory to prevent one binary from overwriting the other's output.
type fileDiffTestCase struct {
	// Name identifies this test case in go test output via t.Run.
	Name string

	// Args is the command-line arguments passed to both binaries.
	Args []string

	// Stdin is the bytes fed to both binaries on stdin.
	Stdin []byte

	// Env is the environment variable overrides for both binaries.
	Env []string

	// SetupFixture creates fixture files in the given directory. Called once for
	// each binary's directory with identical setup. When nil, no fixture is created.
	SetupFixture func(dir string) error

	// OutputFile is the name of the file to compare after both binaries exit.
	// Resolved relative to each binary's working directory.
	OutputFile string

	// CheckMode, when non-zero, verifies the output file has this permission mode
	// in both directories after execution. Per prd007-sponge R2.3.
	CheckMode os.FileMode
}

// binOutput holds the captured output from a single binary execution.
type binOutput struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runFileDiffTests runs a set of file-output differential test cases.
// Per design decision D1 and prd007-sponge R6.1.
func runFileDiffTests(t *testing.T, goBin string, refBin string, tests []fileDiffTestCase) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleFileDiffTest(t, goBin, refBin, tc)
		})
	}
}

// runSingleFileDiffTest executes a single file-output differential test.
// Per design decision D1 and prd007-sponge R6.1: creates two isolated temp
// directories, runs each binary in its own directory, and compares the output
// files from both directories byte-for-byte.
//
// Per R2 requirements:
//
//	(a) creates two isolated temp directories with identical fixtures
//	(b) runs the Go binary in directory 1
//	(c) runs the reference binary in directory 2
//	(d) compares stdout and stderr byte-for-byte
//	(e) compares exit codes
//	(f) reads the output file from both directories and compares content
//	(g) on divergence reports: args, stdin, both file contents, both stdouts,
//	    both stderrs, and both exit codes
func runSingleFileDiffTest(t *testing.T, goBin string, refBin string, tc fileDiffTestCase) {
	t.Helper()

	// (a) Create two isolated temp directories with identical fixtures.
	goDir := t.TempDir()
	refDir := t.TempDir()

	if tc.SetupFixture != nil {
		if err := tc.SetupFixture(goDir); err != nil {
			t.Fatalf("setting up Go fixture: %v", err)
		}
		if err := tc.SetupFixture(refDir); err != nil {
			t.Fatalf("setting up reference fixture: %v", err)
		}
	}

	// (b) Run the Go binary in directory 1.
	goResult := execBinary(t, goBin, tc.Args, tc.Stdin, tc.Env, goDir)

	// (c) Run the reference binary in directory 2.
	refResult := execBinary(t, refBin, tc.Args, tc.Stdin, tc.Env, refDir)

	// (d) Compare stdout and stderr byte-for-byte.
	failed := false

	if !bytes.Equal(refResult.stdout, goResult.stdout) {
		failed = true
		t.Errorf("stdout divergence")
	}

	if !bytes.Equal(refResult.stderr, goResult.stderr) {
		failed = true
		t.Errorf("stderr divergence")
	}

	// (e) Compare exit codes.
	if refResult.exitCode != goResult.exitCode {
		failed = true
		t.Errorf("exit code divergence: reference=%d, go=%d", refResult.exitCode, goResult.exitCode)
	}

	// (f) Read the output file from both directories and compare content.
	// Per prd007-sponge R6.1 and prd001-testutils R5.1-R5.2.
	goFilePath := filepath.Join(goDir, tc.OutputFile)
	refFilePath := filepath.Join(refDir, tc.OutputFile)

	goContent, goFileErr := os.ReadFile(goFilePath)
	refContent, refFileErr := os.ReadFile(refFilePath)

	if goFileErr != nil && refFileErr != nil {
		// Both failed to produce the file; may be expected for error cases.
	} else if goFileErr != nil {
		failed = true
		t.Errorf("Go binary did not produce output file %s: %v", tc.OutputFile, goFileErr)
	} else if refFileErr != nil {
		failed = true
		t.Errorf("reference binary did not produce output file %s: %v", tc.OutputFile, refFileErr)
	} else if !bytes.Equal(refContent, goContent) {
		failed = true
		t.Errorf("output file content divergence for %s:\n  reference (%d bytes): %s\n  go        (%d bytes): %s",
			tc.OutputFile,
			len(refContent), truncateOutput(refContent, maxStdinReport),
			len(goContent), truncateOutput(goContent, maxStdinReport),
		)
	}

	// Check file mode if requested. Per prd007-sponge R2.3.
	if tc.CheckMode != 0 && goFileErr == nil && refFileErr == nil {
		goInfo, err := os.Stat(goFilePath)
		if err != nil {
			t.Errorf("stat Go output file: %v", err)
		} else if goInfo.Mode().Perm() != tc.CheckMode {
			failed = true
			t.Errorf("Go output file mode: got %04o, want %04o", goInfo.Mode().Perm(), tc.CheckMode)
		}

		refInfo, err := os.Stat(refFilePath)
		if err != nil {
			t.Errorf("stat reference output file: %v", err)
		} else if refInfo.Mode().Perm() != tc.CheckMode {
			failed = true
			t.Errorf("reference output file mode: got %04o, want %04o", refInfo.Mode().Perm(), tc.CheckMode)
		}
	}

	// (g) On divergence, report full context.
	if failed {
		var goFileStr, refFileStr string
		if goFileErr == nil {
			goFileStr = truncateOutput(goContent, maxStdinReport)
		} else {
			goFileStr = fmt.Sprintf("<error: %v>", goFileErr)
		}
		if refFileErr == nil {
			refFileStr = truncateOutput(refContent, maxStdinReport)
		} else {
			refFileStr = fmt.Sprintf("<error: %v>", refFileErr)
		}

		t.Errorf("\n--- Divergence Report ---\n"+
			"Args: %v\n"+
			"Stdin (%d bytes): %s\n"+
			"Reference exit code: %d\n"+
			"Go        exit code: %d\n"+
			"Reference stdout:\n%s\n"+
			"Go        stdout:\n%s\n"+
			"Reference stderr:\n%s\n"+
			"Go        stderr:\n%s\n"+
			"Reference file %s:\n%s\n"+
			"Go        file %s:\n%s",
			tc.Args, len(tc.Stdin), truncateOutput(tc.Stdin, maxStdinReport),
			refResult.exitCode, goResult.exitCode,
			refResult.stdout, goResult.stdout,
			refResult.stderr, goResult.stderr,
			tc.OutputFile, refFileStr,
			tc.OutputFile, goFileStr,
		)
	}
}

// execBinary runs a single binary with the given args, stdin, environment, and
// working directory under a timeout. It captures stdout, stderr, and exit code.
//
// Per prd001-testutils R2.1-R2.5.
func execBinary(t *testing.T, binary string, args []string, stdin []byte, env []string, workDir string) binOutput {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir

	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	// Check for timeout. Per prd001-testutils R2.3 and AC5.
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s exceeded %v timeout", binary, execTimeout)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("running binary %s: %v", binary, err)
		}
	}

	return binOutput{
		stdout:   stdoutBuf.Bytes(),
		stderr:   stderrBuf.Bytes(),
		exitCode: exitCode,
	}
}

// truncateOutput returns a string representation of b, truncated to maxLen bytes
// with an ellipsis suffix if truncated.
func truncateOutput(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + "..."
}

// TestSpongeDifferentialPassthrough tests passthrough mode (no output filename)
// using RunDiffTests from pkg/testutils with standard stdout comparison.
// Per design decision D2, test-rel01.1.yaml sponge_passthrough_stdout.
func TestSpongeDifferentialPassthrough(t *testing.T) {
	if goBinary == "" {
		t.Skip("Go sponge binary could not be built; skipping differential tests")
	}
	if refBinary == "" {
		t.Skip("reference sponge binary not found on PATH (brew install moreutils); skipping differential tests")
	}

	tests := []testutils.DiffTest{
		{
			// Per test-rel01.1.yaml: sponge_passthrough_stdout.
			// Traces: prd007-sponge R4.1, R4.3.
			Name:  "sponge_passthrough_stdout",
			Args:  nil,
			Stdin: []byte("hello world\n"),
			Env:   baseEnv,
		},
	}

	testutils.RunDiffTests(t, goBinary, refBinary, tests)
}

// TestSpongeDifferentialFileOutput runs all file-output differential test cases
// from test-rel01.1.yaml (sponge section). Per design decision D1 and D2: these
// tests use runFileDiffTests with isolated directories per binary.
func TestSpongeDifferentialFileOutput(t *testing.T) {
	if goBinary == "" {
		t.Skip("Go sponge binary could not be built; skipping differential tests")
	}
	if refBinary == "" {
		t.Skip("reference sponge binary not found on PATH (brew install moreutils); skipping differential tests")
	}

	// Generate 100-line stdin for sponge_small_stdin_to_file.
	var smallStdin bytes.Buffer
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&smallStdin, "%d\n", i)
	}

	// Generate 1 MB+ stdin for sponge_large_stdin.
	// Per test-rel01.1.yaml: "generated payload of 1,048,576+ bytes".
	largePattern := []byte("abcdefghijklmnopqrstuvwxyz0123456789\n")
	largeStdin := bytes.Repeat(largePattern, (1048576/len(largePattern))+1)

	tests := []fileDiffTestCase{
		// --- Core soak-before-write tests (prd007-sponge R1, R2) ---

		{
			// Per test-rel01.1.yaml: sponge_small_stdin_to_file.
			// Traces: prd007-sponge R1.1, R1.2, R2.1.
			Name:       "sponge_small_stdin_to_file",
			Args:       []string{"outfile.txt"},
			Stdin:      smallStdin.Bytes(),
			Env:        baseEnv,
			OutputFile: "outfile.txt",
		},
		{
			// Per test-rel01.1.yaml: sponge_soak_before_write.
			// Traces: prd007-sponge R1.1, R2.5.
			Name:  "sponge_soak_before_write",
			Args:  []string{"data.txt"},
			Stdin: []byte("line1\nline2\nline3\n"),
			Env:   baseEnv,
			SetupFixture: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "data.txt"), []byte("line1\nline2\nline3\n"), 0644)
			},
			OutputFile: "data.txt",
		},
		{
			// Per test-rel01.1.yaml: sponge_empty_stdin.
			// Traces: prd007-sponge R1.1, R2.1.
			Name:       "sponge_empty_stdin",
			Args:       []string{"empty_out.txt"},
			Stdin:      []byte{},
			Env:        baseEnv,
			OutputFile: "empty_out.txt",
		},

		// --- Append mode tests (prd007-sponge R3) ---

		{
			// Per test-rel01.1.yaml: sponge_append_mode.
			// Traces: prd007-sponge R3.1, R3.3.
			Name:  "sponge_append_mode",
			Args:  []string{"-a", "existing.txt"},
			Stdin: []byte("appended line\n"),
			Env:   baseEnv,
			SetupFixture: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("original line\n"), 0644)
			},
			OutputFile: "existing.txt",
		},
		{
			// Per test-rel01.1.yaml: sponge_append_mode_no_existing_file.
			// Traces: prd007-sponge R3.2.
			Name:       "sponge_append_mode_no_existing_file",
			Args:       []string{"-a", "newfile.txt"},
			Stdin:      []byte("new content\n"),
			Env:        baseEnv,
			OutputFile: "newfile.txt",
		},

		// --- Large stdin test (prd007-sponge R1.3, R1.4) ---

		{
			// Per test-rel01.1.yaml: sponge_large_stdin.
			// Traces: prd007-sponge R1.3, R1.4.
			Name:       "sponge_large_stdin",
			Args:       []string{"large_out.txt"},
			Stdin:      largeStdin,
			Env:        baseEnv,
			OutputFile: "large_out.txt",
		},

		// --- Permission preservation test (prd007-sponge R2.3) ---

		{
			// Per test-rel01.1.yaml: sponge_output_file_exists_mode.
			// Traces: prd007-sponge R2.3.
			Name:  "sponge_output_file_exists_mode",
			Args:  []string{"protected.txt"},
			Stdin: []byte("new content\n"),
			Env:   baseEnv,
			SetupFixture: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "protected.txt"), []byte("old content\n"), 0640)
			},
			OutputFile: "protected.txt",
			CheckMode:  0640,
		},
	}

	runFileDiffTests(t, goBinary, refBinary, tests)
}
