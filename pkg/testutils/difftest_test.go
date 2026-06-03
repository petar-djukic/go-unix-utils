// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func findBinary(t *testing.T, name string, paths ...string) string {
	t.Helper()
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skipf("%s binary not found", name)
	return ""
}

func verifySubprocessFails(
	t *testing.T, testName, envVar string, wantMsgs ...string,
) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+testName+"$")
	cmd.Env = append(os.Environ(), envVar+"=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected subprocess test to fail, but it passed")
	}
	out := string(output)
	for _, msg := range wantMsgs {
		if !strings.Contains(out, msg) {
			t.Fatalf("expected %q in output, got:\n%s", msg, out)
		}
	}
}

func TestRunDiffTests_MatchingOutputs(t *testing.T) {
	t.Parallel()
	echoBin := findBinary(t, "echo", "/bin/echo", "/usr/bin/echo")
	tests := []DiffTest{
		{Name: "simple-arg", Args: []string{"hello"}},
		{Name: "multiple-args", Args: []string{"hello", "world"}},
		{Name: "no-args"},
	}
	RunDiffTests(t, echoBin, echoBin, tests)
}

func TestRunDiffTests_MatchingExitCodes(t *testing.T) {
	t.Parallel()
	trueBin := findBinary(t, "true", "/usr/bin/true", "/bin/true")
	falseBin := findBinary(t, "false", "/usr/bin/false", "/bin/false")
	RunDiffTests(t, trueBin, trueBin, []DiffTest{
		{Name: "true-zero-exit"},
	})
	RunDiffTests(t, falseBin, falseBin, []DiffTest{
		{Name: "false-nonzero-exit"},
	})
}

func TestRunDiffTests_WithStdin(t *testing.T) {
	t.Parallel()
	catBin := findBinary(t, "cat", "/bin/cat", "/usr/bin/cat")
	tests := []DiffTest{
		{Name: "passthrough", Stdin: []byte("hello from stdin\n")},
		{Name: "empty-stdin", Stdin: []byte{}},
		{Name: "multiline", Stdin: []byte("line1\nline2\nline3\n")},
	}
	RunDiffTests(t, catBin, catBin, tests)
}

func TestRunDiffTests_WithEnv(t *testing.T) {
	t.Parallel()
	bin := findBinary(t, "printenv", "/usr/bin/printenv", "/bin/printenv")
	tests := []DiffTest{
		{Name: "lc-all-default", Args: []string{"LC_ALL"}},
		{
			Name: "custom-env",
			Args: []string{"DIFFTEST_CUSTOM"},
			Env:  []string{"DIFFTEST_CUSTOM=test_value"},
		},
	}
	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTests_DivergentStdout(t *testing.T) {
	t.Parallel()
	echoBin := findBinary(t, "echo", "/bin/echo", "/usr/bin/echo")
	trueBin := findBinary(t, "true", "/usr/bin/true", "/bin/true")
	if os.Getenv("TEST_DIVERGENT_STDOUT") == "1" {
		RunDiffTests(t, echoBin, trueBin, []DiffTest{
			{Name: "stdout-differs", Args: []string{"hello"}},
		})
		return
	}
	verifySubprocessFails(t, "TestRunDiffTests_DivergentStdout",
		"TEST_DIVERGENT_STDOUT",
		"divergence detected", "ref stdout", "go  stdout")
}

func TestRunDiffTests_DivergentExitCode(t *testing.T) {
	t.Parallel()
	trueBin := findBinary(t, "true", "/usr/bin/true", "/bin/true")
	falseBin := findBinary(t, "false", "/usr/bin/false", "/bin/false")
	if os.Getenv("TEST_DIVERGENT_EXIT") == "1" {
		RunDiffTests(t, trueBin, falseBin, []DiffTest{
			{Name: "exit-code-differs"},
		})
		return
	}
	verifySubprocessFails(t, "TestRunDiffTests_DivergentExitCode",
		"TEST_DIVERGENT_EXIT",
		"divergence detected", "ref exit", "go  exit")
}

func TestTimestampNormalizer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"syslog-format", "May 13 14:30:00 message", "<TIMESTAMP> message"},
		{"iso-format", "2026-05-13 14:30:00 message", "<TIMESTAMP> message"},
		{"time-only", "14:30:00 message", "<TIMESTAMP> message"},
		{"no-timestamp", "hello world", "hello world"},
		{"empty", "", ""},
		{"embedded", "event at 09:15:30 done", "event at <TIMESTAMP> done"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := string(TimestampNormalizer([]byte(tc.input)))
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestComposeNormalizers(t *testing.T) {
	t.Parallel()
	upper := func(b []byte) []byte { return bytes.ToUpper(b) }
	prefix := func(b []byte) []byte {
		return append([]byte(">>"), b...)
	}
	composed := ComposeNormalizers(upper, prefix)
	got := string(composed([]byte("hello")))
	if got != ">>HELLO" {
		t.Fatalf("got %q, want %q", got, ">>HELLO")
	}
}

func TestComposeNormalizers_Empty(t *testing.T) {
	t.Parallel()
	composed := ComposeNormalizers()
	got := string(composed([]byte("unchanged")))
	if got != "unchanged" {
		t.Fatalf("got %q, want %q", got, "unchanged")
	}
}

func TestBuildBinary(t *testing.T) {
	t.Parallel()
	mainFile := filepath.Join("..", "..", "cmd", "cat", "main.go")
	if _, err := os.Stat(mainFile); os.IsNotExist(err) {
		t.Skip("cmd/cat not yet generated")
	}
	binPath := BuildBinary(t, "../../cmd/cat")
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("binary does not exist: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("binary is not executable")
	}
}

func TestBuildBinary_Failure(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_BUILD_FAILURE") == "1" {
		BuildBinary(t, "./nonexistent-pkg-abc123")
		return
	}
	verifySubprocessFails(t, "TestBuildBinary_Failure",
		"TEST_BUILD_FAILURE", "BuildBinary")
}
