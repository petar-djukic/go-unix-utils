// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// buildMockBinary compiles a small Go program from source into a temporary
// binary and returns its path.
func buildMockBinary(t *testing.T, name, source string) string {
	t.Helper()
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcPath, []byte(source), 0o644); err != nil {
		t.Fatalf("writing mock source: %v", err)
	}
	modPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(modPath, []byte("module mock\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("writing mock go.mod: %v", err)
	}
	binPath := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("building mock binary %s: %v\n%s", name, err, out)
	}
	return binPath
}

// mockEchoSource prints "hello" to stdout and exits 0.
const mockEchoSource = `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`

// mockDivergentSource prints "world" to stdout and exits 0.
const mockDivergentSource = `package main

import "fmt"

func main() {
	fmt.Println("world")
}
`

// mockExitOneSource exits with code 1.
const mockExitOneSource = `package main

import "os"

func main() {
	os.Exit(1)
}
`

// mockStderrSource writes "err" to stderr and exits 0.
const mockStderrSource = `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "err")
}
`

// mockEnvPrinterSource prints the value of LC_ALL.
const mockEnvPrinterSource = `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println(os.Getenv("LC_ALL"))
}
`

// mockPwdSource prints the working directory.
const mockPwdSource = `package main

import (
	"fmt"
	"os"
)

func main() {
	wd, _ := os.Getwd()
	fmt.Println(wd)
}
`

// mockFileWriterSource writes "output" to a file named "out.txt" in the
// working directory.
const mockFileWriterSource = `package main

import "os"

func main() {
	os.WriteFile("out.txt", []byte("output"), 0o644)
}
`

// mockBadFileWriterSource writes "wrong" to "out.txt" in the working directory.
const mockBadFileWriterSource = `package main

import "os"

func main() {
	os.WriteFile("out.txt", []byte("wrong"), 0o644)
}
`

// mockTimestampSource prints a line with a timestamp pattern.
const mockTimestampSource = `package main

import "fmt"

func main() {
	fmt.Println("event at 2026-03-09 14:30:00 happened")
}
`

// mockTimestampAltSource prints a line with a different timestamp.
const mockTimestampAltSource = `package main

import "fmt"

func main() {
	fmt.Println("event at 2026-03-09 15:45:22 happened")
}
`

// mockNoopSource does nothing and exits 0.
const mockNoopSource = `package main

func main() {}
`

func TestDiffTestZeroValue(t *testing.T) {
	t.Parallel()
	bin := buildMockBinary(t, "noop", mockNoopSource)
	RunDiffTests(t, bin, bin, []DiffTest{{Name: "zero"}})
}

func TestRunDiffTestsMatchingOutput(t *testing.T) {
	t.Parallel()
	bin := buildMockBinary(t, "echo", mockEchoSource)
	tests := []DiffTest{
		{Name: "matching", ExitCode: 0},
	}
	RunDiffTests(t, bin, bin, tests)
}

// TestRunDiffTestsDivergentStdout verifies that RunDiffTests reports failure
// when stdout differs between binaries. Uses subprocess pattern to capture
// test failure without crashing the parent.
func TestRunDiffTestsDivergentStdout(t *testing.T) {
	t.Parallel()
	// Subprocess mode: run the actual divergent test
	if os.Getenv("TEST_SUBPROCESS_DIVERGENT_STDOUT") == "1" {
		goBin := buildMockBinary(t, "go-bin", mockDivergentSource)
		refBin := buildMockBinary(t, "ref-bin", mockEchoSource)
		RunDiffTests(t, goBin, refBin, []DiffTest{
			{Name: "divergent-stdout", ExitCode: 0},
		})
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRunDiffTestsDivergentStdout$", "-test.v")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS_DIVERGENT_STDOUT=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess test to fail on divergent stdout")
	}
	if !strings.Contains(string(out), "divergence detected") {
		t.Errorf("expected failure message to contain 'divergence detected', got:\n%s", out)
	}
}

// TestRunDiffTestsDivergentExitCode verifies failure on exit code mismatch.
func TestRunDiffTestsDivergentExitCode(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_SUBPROCESS_DIVERGENT_EXIT") == "1" {
		goBin := buildMockBinary(t, "go-bin", mockExitOneSource)
		refBin := buildMockBinary(t, "ref-bin", mockNoopSource)
		RunDiffTests(t, goBin, refBin, []DiffTest{
			{Name: "divergent-exit", ExitCode: 0},
		})
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRunDiffTestsDivergentExitCode$", "-test.v")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS_DIVERGENT_EXIT=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess test to fail on divergent exit code")
	}
	if !strings.Contains(string(out), "divergence detected") {
		t.Errorf("expected failure message to contain 'divergence detected', got:\n%s", out)
	}
}

// TestRunDiffTestsDivergentStderr verifies failure on stderr mismatch.
func TestRunDiffTestsDivergentStderr(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_SUBPROCESS_DIVERGENT_STDERR") == "1" {
		goBin := buildMockBinary(t, "go-bin", mockStderrSource)
		refBin := buildMockBinary(t, "ref-bin", mockNoopSource)
		RunDiffTests(t, goBin, refBin, []DiffTest{
			{Name: "divergent-stderr", ExitCode: 0},
		})
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRunDiffTestsDivergentStderr$", "-test.v")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS_DIVERGENT_STDERR=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess test to fail on divergent stderr")
	}
	if !strings.Contains(string(out), "divergence detected") {
		t.Errorf("expected failure message to contain 'divergence detected', got:\n%s", out)
	}
}

func TestEnvDefaultLCALL(t *testing.T) {
	t.Parallel()
	bin := buildMockBinary(t, "env-printer", mockEnvPrinterSource)
	// Both binaries print LC_ALL; harness sets LC_ALL=C so output is "C\n"
	tests := []DiffTest{
		{Name: "lc-all-default", ExitCode: 0},
	}
	RunDiffTests(t, bin, bin, tests)
}

func TestEnvOverrideLCALL(t *testing.T) {
	t.Parallel()
	bin := buildMockBinary(t, "env-printer", mockEnvPrinterSource)
	tests := []DiffTest{
		{
			Name:     "lc-all-override",
			Env:      []string{"LC_ALL=en_US.UTF-8"},
			ExitCode: 0,
		},
	}
	RunDiffTests(t, bin, bin, tests)
}

func TestWorkDirDefault(t *testing.T) {
	t.Parallel()
	bin := buildMockBinary(t, "pwd", mockPwdSource)
	// WorkDir empty → t.TempDir(). Both binaries run in the same temp dir.
	tests := []DiffTest{
		{Name: "workdir-default", ExitCode: 0},
	}
	RunDiffTests(t, bin, bin, tests)
}

func TestWorkDirExplicit(t *testing.T) {
	t.Parallel()
	bin := buildMockBinary(t, "pwd", mockPwdSource)
	dir := t.TempDir()
	tests := []DiffTest{
		{Name: "workdir-explicit", WorkDir: dir, ExitCode: 0},
	}
	RunDiffTests(t, bin, bin, tests)
}

func TestExpectedFilesMatch(t *testing.T) {
	t.Parallel()
	bin := buildMockBinary(t, "file-writer", mockFileWriterSource)
	tests := []DiffTest{
		{
			Name:          "files-match",
			ExitCode:      0,
			ExpectedFiles: map[string][]byte{"out.txt": []byte("output")},
		},
	}
	RunDiffTests(t, bin, bin, tests)
}

// TestExpectedFilesDiverge verifies failure on file content mismatch.
func TestExpectedFilesDiverge(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_SUBPROCESS_FILES_DIVERGE") == "1" {
		goBin := buildMockBinary(t, "bad-writer", mockBadFileWriterSource)
		refBin := buildMockBinary(t, "file-writer", mockFileWriterSource)
		RunDiffTests(t, goBin, refBin, []DiffTest{
			{
				Name:          "files-diverge",
				ExitCode:      0,
				ExpectedFiles: map[string][]byte{"out.txt": []byte("output")},
			},
		})
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestExpectedFilesDiverge$", "-test.v")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS_FILES_DIVERGE=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess test to fail on divergent file content")
	}
	if !strings.Contains(string(out), "ExpectedFiles divergence") {
		t.Errorf("expected failure message to contain 'ExpectedFiles divergence', got:\n%s", out)
	}
}

func TestTimestampNormalizer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "iso-datetime",
			input: "event at 2026-03-09 14:30:00 happened",
			want:  "event at <TIMESTAMP> happened",
		},
		{
			name:  "syslog-format",
			input: "Feb 19 12:34:56 some event",
			want:  "<TIMESTAMP> some event",
		},
		{
			name:  "time-only",
			input: "at 12:34:56 done",
			want:  "at <TIMESTAMP> done",
		},
		{
			name:  "no-timestamp",
			input: "no timestamp here",
			want:  "no timestamp here",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := string(TimestampNormalizer([]byte(tc.input)))
			if got != tc.want {
				t.Errorf("TimestampNormalizer(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestTimestampNormalizerInRunDiffTests(t *testing.T) {
	t.Parallel()
	goBin := buildMockBinary(t, "ts1", mockTimestampSource)
	refBin := buildMockBinary(t, "ts2", mockTimestampAltSource)

	// Without normalization these would diverge; with TimestampNormalizer they match.
	tests := []DiffTest{
		{
			Name:      "timestamp-normalized",
			ExitCode:  0,
			Normalize: []NormalizeFunc{TimestampNormalizer},
		},
	}
	RunDiffTests(t, goBin, refBin, tests)
}

func TestComposeNormalizersEmpty(t *testing.T) {
	t.Parallel()
	result := ComposeNormalizers()
	if result == nil {
		t.Fatal("ComposeNormalizers() should return identity function, not nil")
	}
	input := []byte("hello world")
	got := result(input)
	if string(got) != string(input) {
		t.Errorf("identity function: got %q, want %q", got, input)
	}
}

func TestComposeNormalizersChaining(t *testing.T) {
	t.Parallel()

	upper := func(b []byte) []byte {
		out := make([]byte, len(b))
		for i, c := range b {
			if c >= 'a' && c <= 'z' {
				out[i] = c - 32
			} else {
				out[i] = c
			}
		}
		return out
	}
	addSuffix := func(b []byte) []byte {
		return append(b, []byte("-DONE")...)
	}

	composed := ComposeNormalizers(upper, addSuffix)
	result := composed([]byte("hello"))
	expected := []byte("HELLO-DONE")
	if string(result) != string(expected) {
		t.Errorf("ComposeNormalizers result = %q, want %q", result, expected)
	}
}

func TestBuildBinary(t *testing.T) {
	t.Parallel()
	// Create a mini Go module in a temp dir so BuildBinary can compile it.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testmod\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("writing source: %v", err)
	}

	binPath := BuildBinary(t, dir)
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("BuildBinary returned path that does not exist: %v", err)
	}

	// Verify binary is executable
	if runtime.GOOS != "windows" {
		if info.Mode()&0o111 == 0 {
			t.Error("BuildBinary output is not executable")
		}
	}
}

func TestBuildEnv(t *testing.T) {
	t.Parallel()

	// Verify buildEnv sets LC_ALL=C by default
	env := buildEnv(nil)
	if !slices.Contains(env, "LC_ALL=C") {
		t.Error("buildEnv(nil) does not set LC_ALL=C")
	}

	// Verify user override replaces default
	env = buildEnv([]string{"LC_ALL=en_US.UTF-8"})
	if slices.Contains(env, "LC_ALL=C") {
		t.Error("buildEnv should have overridden LC_ALL=C with user value")
	}
	if !slices.Contains(env, "LC_ALL=en_US.UTF-8") {
		t.Error("buildEnv did not apply user LC_ALL override")
	}
}
