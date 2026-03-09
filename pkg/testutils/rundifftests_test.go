// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildMockBinary compiles a Go source file into a temporary binary and returns its path.
func buildMockBinary(t *testing.T, source string) string {
	t.Helper()

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "main.go")
	binPath := filepath.Join(tmpDir, "mock")

	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("writing mock source: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", binPath, srcPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("building mock binary: %v", err)
	}

	return binPath
}

// echoStdoutSource is a mock binary that prints its args joined by spaces to stdout.
const echoStdoutSource = `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Println(strings.Join(os.Args[1:], " "))
}
`

// exitCodeSource is a mock binary that exits with exit code 1.
const exitCodeSource = `package main

import "os"

func main() {
	os.Exit(1)
}
`

// stdinEchoSource is a mock binary that reads stdin and echoes it to stdout.
const stdinEchoSource = `package main

import (
	"io"
	"os"
)

func main() {
	io.Copy(os.Stdout, os.Stdin)
}
`

func TestRunDiffTests_MatchingBinaries(t *testing.T) {
	t.Parallel()

	bin := buildMockBinary(t, echoStdoutSource)

	tests := []DiffTest{
		{
			Name: "basic_args",
			Args: []string{"hello", "world"},
		},
		{
			Name: "no_args",
			Args: nil,
		},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_SubtestIsolation(t *testing.T) {
	t.Parallel()

	bin := buildMockBinary(t, echoStdoutSource)

	tests := []DiffTest{
		{Name: "first", Args: []string{"a"}},
		{Name: "second", Args: []string{"b"}},
		{Name: "third", Args: []string{"c"}},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_StdinPassthrough(t *testing.T) {
	t.Parallel()

	bin := buildMockBinary(t, stdinEchoSource)

	tests := []DiffTest{
		{
			Name:  "stdin_data",
			Stdin: []byte("hello from stdin\n"),
		},
		{
			Name:  "empty_stdin",
			Stdin: []byte{},
		},
		{
			Name:  "nil_stdin",
			Stdin: nil,
		},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_ExitCodeMatch(t *testing.T) {
	t.Parallel()

	bin := buildMockBinary(t, exitCodeSource)

	tests := []DiffTest{
		{
			Name:     "exit_one",
			ExitCode: 1,
		},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_Normalizers(t *testing.T) {
	t.Parallel()

	bin := buildMockBinary(t, echoStdoutSource)

	// Normalizer that uppercases output
	upper := func(data []byte) []byte {
		result := make([]byte, len(data))
		for i, b := range data {
			if b >= 'a' && b <= 'z' {
				result[i] = b - 32
			} else {
				result[i] = b
			}
		}
		return result
	}

	tests := []DiffTest{
		{
			Name:      "with_normalizer",
			Args:      []string{"hello"},
			Normalize: []NormalizeFunc{upper},
		},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_WorkDir(t *testing.T) {
	t.Parallel()

	bin := buildMockBinary(t, echoStdoutSource)
	workDir := t.TempDir()

	tests := []DiffTest{
		{
			Name:    "custom_workdir",
			Args:    []string{"test"},
			WorkDir: workDir,
		},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_EnvOverride(t *testing.T) {
	t.Parallel()

	// Mock binary that prints the LC_ALL env var
	source := `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println(os.Getenv("LC_ALL"))
}
`
	bin := buildMockBinary(t, source)

	tests := []DiffTest{
		{
			Name: "default_lc_all",
		},
		{
			Name: "override_lc_all",
			Env:  []string{"LC_ALL=en_US.UTF-8"},
		},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestBuildEnv_DefaultLCALL(t *testing.T) {
	t.Parallel()

	env := buildEnv(nil)
	found := false
	for _, kv := range env {
		if kv == "LC_ALL=C" {
			found = true
			break
		}
	}
	if !found {
		t.Error("buildEnv(nil) should set LC_ALL=C")
	}
}

func TestBuildEnv_OverrideLCALL(t *testing.T) {
	t.Parallel()

	env := buildEnv([]string{"LC_ALL=en_US.UTF-8"})
	found := false
	for _, kv := range env {
		if kv == "LC_ALL=en_US.UTF-8" {
			found = true
		}
		if kv == "LC_ALL=C" {
			t.Error("buildEnv should not have LC_ALL=C when overridden")
		}
	}
	if !found {
		t.Error("buildEnv should have LC_ALL=en_US.UTF-8 when overridden")
	}
}

func TestTruncateStdin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"nil", nil, "<nil>"},
		{"short", []byte("hello"), `"hello"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncateStdin(tc.input)
			if got != tc.want {
				t.Errorf("truncateStdin(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
