// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildBinary compiles the paste binary into a temp directory and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "paste")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = filepath.Join(findModuleRoot(t), "cmd", "paste")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return binPath
}

// findModuleRoot walks up from the current directory to find go.mod.
func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find module root")
		}
		dir = parent
	}
}

type diffTest struct {
	name  string
	args  []string
	stdin []byte
	env   []string
	// setup creates temp files and returns a cleanup function.
	// The returned map holds filename -> path mappings for arg substitution.
	setup func(t *testing.T) map[string]string
}

// runBinary executes a binary with args, stdin, and env, returning stdout, stderr, and exit code.
func runBinary(t *testing.T, bin string, args []string, stdin []byte, env []string) ([]byte, []byte, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", bin, err)
		}
	}
	return stdout.Bytes(), stderr.Bytes(), exitCode
}

// writeTestFile creates a temp file with the given content and returns its path.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// substituteArgs replaces filename placeholders with actual temp file paths.
func substituteArgs(args []string, pathMap map[string]string) []string {
	result := make([]string, len(args))
	for i, a := range args {
		if p, ok := pathMap[a]; ok {
			result[i] = p
		} else {
			result[i] = a
		}
	}
	return result
}

func TestDiff(t *testing.T) {
	goBin := buildBinary(t)
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	tests := []diffTest{
		{
			name:  "two_files_default",
			args:  []string{"file1.txt", "file2.txt"},
			env:   []string{"LC_ALL=C"},
			setup: setupTwoEqualFiles,
		},
		{
			name:  "custom_delimiter",
			args:  []string{"-d:", "file1.txt", "file2.txt"},
			env:   []string{"LC_ALL=C"},
			setup: setupTwoEqualFiles,
		},
		{
			name:  "serial_mode",
			args:  []string{"-s", "file1.txt"},
			env:   []string{"LC_ALL=C"},
			setup: setupTwoEqualFiles,
		},
		{
			name: "unequal_length",
			args: []string{"file1.txt", "file2.txt"},
			env:  []string{"LC_ALL=C"},
			setup: func(t *testing.T) map[string]string {
				t.Helper()
				dir := t.TempDir()
				return map[string]string{
					"file1.txt": writeTestFile(t, dir, "file1.txt", "a\nb\nc\n"),
					"file2.txt": writeTestFile(t, dir, "file2.txt", "1\n"),
				}
			},
		},
		{
			name:  "stdin_dash",
			args:  []string{"-d:", "-", "file2.txt"},
			stdin: []byte("x\ny\n"),
			env:   []string{"LC_ALL=C"},
			setup: setupTwoEqualFiles,
		},
		{
			name:  "delimiter_cycling",
			args:  []string{"-d,:", "file1.txt", "file2.txt", "file3.txt"},
			env:   []string{"LC_ALL=C"},
			setup: setupThreeFiles,
		},
		{
			name:  "serial_multiple_files",
			args:  []string{"-s", "file1.txt", "file2.txt"},
			env:   []string{"LC_ALL=C"},
			setup: setupTwoEqualFiles,
		},
		{
			name:  "stdin_only",
			args:  []string{},
			stdin: []byte("hello\nworld\n"),
			env:   []string{"LC_ALL=C"},
		},
		{
			name:  "empty_file",
			args:  []string{"file1.txt", "file2.txt"},
			env:   []string{"LC_ALL=C"},
			setup: func(t *testing.T) map[string]string {
				t.Helper()
				dir := t.TempDir()
				return map[string]string{
					"file1.txt": writeTestFile(t, dir, "file1.txt", "a\nb\n"),
					"file2.txt": writeTestFile(t, dir, "file2.txt", ""),
				}
			},
		},
		{
			name:  "serial_delimiter_cycling",
			args:  []string{"-s", "-d,:", "file1.txt"},
			env:   []string{"LC_ALL=C"},
			setup: func(t *testing.T) map[string]string {
				t.Helper()
				dir := t.TempDir()
				return map[string]string{
					"file1.txt": writeTestFile(t, dir, "file1.txt", "a\nb\nc\nd\n"),
				}
			},
		},
		{
			name:  "backslash_escape_tab",
			args:  []string{`-d\t`, "file1.txt", "file2.txt"},
			env:   []string{"LC_ALL=C"},
			setup: setupTwoEqualFiles,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var pathMap map[string]string
			if tc.setup != nil {
				pathMap = tc.setup(t)
			}

			args := substituteArgs(tc.args, pathMap)

			goOut, goErr, goExit := runBinary(t, goBin, args, tc.stdin, tc.env)
			refOut, refErr, refExit := runBinary(t, refBin, args, tc.stdin, tc.env)

			if !bytes.Equal(goOut, refOut) {
				t.Errorf("stdout mismatch\ngo:  %q\nref: %q", goOut, refOut)
			}
			if goExit != refExit {
				t.Errorf("exit code mismatch: go=%d ref=%d\ngo stderr: %s\nref stderr: %s",
					goExit, refExit, goErr, refErr)
			}
		})
	}
}

// setupTwoEqualFiles creates file1.txt with "a\nb\n" and file2.txt with "1\n2\n".
func setupTwoEqualFiles(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	return map[string]string{
		"file1.txt": writeTestFile(t, dir, "file1.txt", "a\nb\n"),
		"file2.txt": writeTestFile(t, dir, "file2.txt", "1\n2\n"),
	}
}

// setupThreeFiles creates three files for delimiter cycling tests.
func setupThreeFiles(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	return map[string]string{
		"file1.txt": writeTestFile(t, dir, "file1.txt", "a\nb\n"),
		"file2.txt": writeTestFile(t, dir, "file2.txt", "1\n2\n"),
		"file3.txt": writeTestFile(t, dir, "file3.txt", "a\nb\n"),
	}
}
