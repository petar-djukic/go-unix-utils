// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sponge covering prd007-sponge R1-R4.
// Verifies Go sponge against Homebrew moreutils gsponge.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew moreutils reference binary for sponge.
const refBinaryName = "gsponge"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// R1: stdin buffering — small input to file
	smallStdin := generateLines(1, 100)

	// R1: stdin buffering — large input (>1 MB) to file
	largeStdin := generateLines(1, 50000)

	// R3: atomic write — read and write same file content
	soakContent := []byte("line1\nline2\nline3\n")

	tests := []testutils.DiffTest{
		// R1: stdin buffering — write stdin to output file
		{
			Name:          "small_stdin_to_file",
			Args:          []string{"outfile.txt"},
			Stdin:         smallStdin,
			ExpectedFiles: map[string][]byte{"outfile.txt": smallStdin},
		},
		{
			Name:          "large_stdin_to_file",
			Args:          []string{"large_out.txt"},
			Stdin:         largeStdin,
			ExpectedFiles: map[string][]byte{"large_out.txt": largeStdin},
		},
		{
			Name:          "empty_stdin_to_file",
			Args:          []string{"empty_out.txt"},
			Stdin:         []byte{},
			ExpectedFiles: map[string][]byte{"empty_out.txt": {}},
		},

		// R3: atomic write — soak before write (same file read+write)
		{
			Name:          "soak_before_write",
			Args:          []string{"data.txt"},
			Stdin:         soakContent,
			ExpectedFiles: map[string][]byte{"data.txt": soakContent},
		},

		// R1/R4: passthrough to stdout when no filename given
		{
			Name:  "passthrough_stdout",
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "passthrough_empty_stdin",
			Stdin: []byte{},
		},

		// R4: exit code 0 on success
		{
			Name:  "exit_code_success",
			Args:  []string{"ok.txt"},
			Stdin: []byte("data\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)

	// R2: append mode tests run separately because RunDiffTests executes both
	// binaries in the same WorkDir sequentially. The reference binary appends
	// first, then the Go binary appends again, doubling the appended content.
	// We test append mode by running each binary in its own temp directory and
	// comparing results.
	t.Run("append_mode_existing_file", func(t *testing.T) {
		t.Parallel()
		runAppendTest(t, goBin, refBin,
			"original line\n",
			[]byte("appended line\n"),
			[]string{"-a", "existing.txt"},
			"existing.txt",
		)
	})

	t.Run("append_mode_no_existing_file", func(t *testing.T) {
		t.Parallel()
		runAppendTest(t, goBin, refBin,
			"", // no pre-existing file
			[]byte("new content\n"),
			[]string{"-a", "newfile.txt"},
			"newfile.txt",
		)
	})
}

// runAppendTest runs goBin and refBin each in their own temp directory with
// identical preconditions and compares exit codes, stdout, stderr, and the
// resulting file content.
func runAppendTest(t *testing.T, goBin, refBin, preExisting string, stdin []byte, args []string, outFile string) {
	t.Helper()

	type result struct {
		stdout   []byte
		stderr   []byte
		exitCode int
		content  []byte
	}

	run := func(binary, label string) result {
		t.Helper()
		dir := t.TempDir()
		if preExisting != "" {
			writeFixture(t, dir, outFile, preExisting)
		}
		cmd := exec.Command(binary, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		cmd.Stdin = strings.NewReader(string(stdin))
		var stdoutBuf, stderrBuf strings.Builder
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
		exitCode := 0
		if err := cmd.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				exitCode = ee.ExitCode()
			} else {
				t.Fatalf("running %s (%s): %v", label, binary, err)
			}
		}
		content, readErr := os.ReadFile(filepath.Join(dir, outFile))
		if readErr != nil {
			t.Fatalf("reading output file after %s: %v", label, readErr)
		}
		return result{
			stdout:   []byte(stdoutBuf.String()),
			stderr:   []byte(stderrBuf.String()),
			exitCode: exitCode,
			content:  content,
		}
	}

	ref := run(refBin, "reference")
	goR := run(goBin, "go")

	if ref.exitCode != goR.exitCode {
		t.Errorf("exit code mismatch: reference=%d, go=%d", ref.exitCode, goR.exitCode)
	}
	if string(ref.stdout) != string(goR.stdout) {
		t.Errorf("stdout mismatch:\n  reference: %q\n  go:        %q", ref.stdout, goR.stdout)
	}
	if string(ref.stderr) != string(goR.stderr) {
		t.Errorf("stderr mismatch:\n  reference: %q\n  go:        %q", ref.stderr, goR.stderr)
	}
	if string(ref.content) != string(goR.content) {
		t.Errorf("file content mismatch:\n  reference: %q\n  go:        %q", ref.content, goR.content)
	}
}

// writeFixture creates a file in dir with the given name and content.
func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture %s: %v", name, err)
	}
}

// generateLines returns numbered lines "1\n2\n...\nN\n" for the range [start, end].
func generateLines(start, end int) []byte {
	var b strings.Builder
	for i := start; i <= end; i++ {
		fmt.Fprintf(&b, "%d\n", i)
	}
	return []byte(b.String())
}
