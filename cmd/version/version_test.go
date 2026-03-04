// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Unit tests for cmd/version covering output format, default version value,
// and flag handling.
//
// Implements prd011-magefiles R1.1, R1.2, R1.4.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// goBinary is the path to the compiled Go version binary. Set by TestMain.
var goBinary string

// TestMain builds the Go version binary and cleans up after all tests complete.
// D2: build Go version binary into a temporary directory.
func TestMain(m *testing.M) {
	binDir, err := os.MkdirTemp("", "version-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating bin dir: %v\n", err)
		os.Exit(1)
	}

	goBinary = filepath.Join(binDir, "version")
	cmd := exec.Command("go", "build", "-o", goBinary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building Go version binary: %v\n%s", err, out)
		os.RemoveAll(binDir) // best-effort cleanup
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(binDir) // best-effort cleanup
	os.Exit(code)
}

// runVersion executes the version binary with the given args and returns
// stdout, stderr, and exit code.
func runVersion(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command(goBinary, args...)
	var outBuf, errBuf []byte
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("creating stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("starting version binary: %v", err)
	}

	outBuf, _ = readAll(outPipe)
	errBuf, _ = readAll(errPipe)

	err = cmd.Wait()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("waiting for version binary: %v", err)
		}
	}

	return string(outBuf), string(errBuf), exitCode
}

// readAll reads all bytes from an io.Reader.
func readAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 1024)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}

// TestVersionNoArgs verifies R1.1, R1.2: running the version binary with no
// arguments outputs exactly "dev\n" to stdout, produces empty stderr, and
// exits with code 0.
func TestVersionNoArgs(t *testing.T) {
	t.Parallel()

	stdout, stderr, exitCode := runVersion(t)

	if stdout != "dev\n" {
		t.Errorf("stdout: got %q, want %q", stdout, "dev\n")
	}
	if stderr != "" {
		t.Errorf("stderr: got %q, want empty", stderr)
	}
	if exitCode != 0 {
		t.Errorf("exit code: got %d, want 0", exitCode)
	}
}

// TestVersionFlags verifies R1.4: --version and -v each produce stdout output
// identical to the no-args invocation, with empty stderr and exit code 0.
func TestVersionFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{"version-long-flag", []string{"--version"}},
		{"version-short-flag", []string{"-v"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout, stderr, exitCode := runVersion(t, tc.args...)

			if stdout != "dev\n" {
				t.Errorf("stdout: got %q, want %q", stdout, "dev\n")
			}
			if stderr != "" {
				t.Errorf("stderr: got %q, want empty", stderr)
			}
			if exitCode != 0 {
				t.Errorf("exit code: got %d, want 0", exitCode)
			}
		})
	}
}
