// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mockBinPath builds a small Go program and returns the compiled binary path.
func mockBinPath(t *testing.T, name, src string) string {
	t.Helper()
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatalf("write mock source: %v", err)
	}
	binPath := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-o", binPath, srcPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build mock %s: %v\n%s", name, err, stderr.String())
	}
	return binPath
}

// echoStdoutSrc prints its args joined by spaces to stdout.
const echoStdoutSrc = `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Print(strings.Join(os.Args[1:], " "))
}
`

// exitCodeSrc exits with the code given as the first argument.
const exitCodeSrc = `package main

import (
	"os"
	"strconv"
)

func main() {
	if len(os.Args) > 1 {
		code, _ := strconv.Atoi(os.Args[1])
		os.Exit(code)
	}
}
`

// catStdinSrc copies stdin to stdout.
const catStdinSrc = `package main

import (
	"io"
	"os"
)

func main() {
	io.Copy(os.Stdout, os.Stdin)
}
`

func TestRunDiffTests_IdenticalOutput(t *testing.T) {
	t.Parallel()
	bin := mockBinPath(t, "echo", echoStdoutSrc)
	tests := []DiffTest{
		{Name: "hello", Args: []string{"hello"}, ExitCode: 0},
		{Name: "empty", Args: nil, ExitCode: 0},
	}
	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_StdinPassthrough(t *testing.T) {
	t.Parallel()
	bin := mockBinPath(t, "cat", catStdinSrc)
	tests := []DiffTest{
		{Name: "stdin-data", Stdin: []byte("hello\nworld\n"), ExitCode: 0},
		{Name: "empty-stdin", Stdin: []byte{}, ExitCode: 0},
	}
	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_ExitCodeMatch(t *testing.T) {
	t.Parallel()
	bin := mockBinPath(t, "exitcode", exitCodeSrc)
	tests := []DiffTest{
		{Name: "exit-0", Args: []string{"0"}, ExitCode: 0},
		{Name: "exit-1", Args: []string{"1"}, ExitCode: 1},
		{Name: "exit-42", Args: []string{"42"}, ExitCode: 42},
	}
	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_SubtestNames(t *testing.T) {
	t.Parallel()
	bin := mockBinPath(t, "echo", echoStdoutSrc)
	// Verify each DiffTest runs as a named subtest.
	tests := []DiffTest{
		{Name: "first", Args: []string{"a"}, ExitCode: 0},
		{Name: "second", Args: []string{"b"}, ExitCode: 0},
		{Name: "third", Args: []string{"c"}, ExitCode: 0},
	}
	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_Normalizer(t *testing.T) {
	t.Parallel()
	// Two binaries that produce different output but normalize to the same.
	src1 := `package main
import "fmt"
func main() { fmt.Print("AAA-output") }
`
	src2 := `package main
import "fmt"
func main() { fmt.Print("BBB-output") }
`
	bin1 := mockBinPath(t, "bin1", src1)
	bin2 := mockBinPath(t, "bin2", src2)

	// With normalizers that replace both prefixes to XXX, they match.
	stripAAA := func(data []byte) []byte {
		return bytes.ReplaceAll(data, []byte("AAA"), []byte("XXX"))
	}
	stripBBB := func(data []byte) []byte {
		return bytes.ReplaceAll(data, []byte("BBB"), []byte("XXX"))
	}
	tests := []DiffTest{
		{
			Name:      "normalized-match",
			ExitCode:  0,
			Normalize: []NormalizeFunc{stripAAA, stripBBB},
		},
	}
	RunDiffTests(t, bin1, bin2, tests)
}

func TestRunDiffTests_WorkDir(t *testing.T) {
	t.Parallel()
	// Binary that prints its working directory.
	src := `package main
import (
	"fmt"
	"os"
)
func main() {
	wd, _ := os.Getwd()
	fmt.Print(wd)
}
`
	bin := mockBinPath(t, "pwd", src)
	workDir := t.TempDir()
	tests := []DiffTest{
		{Name: "custom-workdir", WorkDir: workDir, ExitCode: 0},
	}
	RunDiffTests(t, bin, bin, tests)
}

func TestBuildEnv_DefaultLC(t *testing.T) {
	t.Parallel()
	env := buildEnv(nil)
	found := false
	for _, e := range env {
		if e == "LC_ALL=C" {
			found = true
		}
	}
	if !found {
		t.Error("buildEnv did not set LC_ALL=C")
	}
}

func TestBuildEnv_CustomOverride(t *testing.T) {
	t.Parallel()
	env := buildEnv([]string{"LC_ALL=en_US.UTF-8", "FOO=bar"})
	lastLC := ""
	hasFoo := false
	for _, e := range env {
		if strings.HasPrefix(e, "LC_ALL=") {
			lastLC = e
		}
		if e == "FOO=bar" {
			hasFoo = true
		}
	}
	if lastLC != "LC_ALL=en_US.UTF-8" {
		t.Errorf("expected last LC_ALL=en_US.UTF-8, got %q", lastLC)
	}
	if !hasFoo {
		t.Error("custom FOO=bar not found in env")
	}
}

func TestExitCodeFromErr(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"generic-error", fmt.Errorf("some error"), -1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := exitCodeFromErr(tc.err)
			if got != tc.want {
				t.Errorf("exitCodeFromErr(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestTruncateStdin(t *testing.T) {
	t.Parallel()
	short := []byte("hello")
	if got := truncateStdin(short); !bytes.Equal(got, short) {
		t.Errorf("truncateStdin should not truncate short input")
	}
	long := bytes.Repeat([]byte("x"), maxStdinDisplay+100)
	got := truncateStdin(long)
	if len(got) != maxStdinDisplay {
		t.Errorf("truncateStdin length = %d, want %d", len(got), maxStdinDisplay)
	}
}

func TestResolvePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		path    string
		workDir string
		want    string
	}{
		{"absolute", "/tmp/file.txt", "/work", "/tmp/file.txt"},
		{"relative", "output.txt", "/work", "/work/output.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := resolvePath(tc.path, tc.workDir)
			if got != tc.want {
				t.Errorf("resolvePath(%q, %q) = %q, want %q",
					tc.path, tc.workDir, got, tc.want)
			}
		})
	}
}
