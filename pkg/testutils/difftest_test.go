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
	"time"
)

// buildMockScript writes a shell script to a temp directory and returns its path.
func buildMockScript(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "mock.sh")
	content := "#!/bin/sh\n" + script + "\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write mock script: %v", err)
	}
	return path
}

func TestTimestampNormalizer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"syslog_format", "Feb 19 12:34:56 message", "<TIMESTAMP> message"},
		{"iso_format", "2024-02-19 12:34:56 event", "<TIMESTAMP> event"},
		{"time_only", "event at 12:34:56", "event at <TIMESTAMP>"},
		{"no_timestamp", "plain text", "plain text"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := string(TimestampNormalizer([]byte(tc.input)))
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestComposeNormalizers(t *testing.T) {
	t.Parallel()
	fn1 := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("a"), []byte("b")) }
	fn2 := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("b"), []byte("c")) }
	composed := ComposeNormalizers(fn1, fn2)

	// "abc" → fn1 → "bbc" → fn2 → "ccc"
	got := string(composed([]byte("abc")))
	if got != "ccc" {
		t.Errorf("got %q, want %q", got, "ccc")
	}
}

func TestComposeNormalizersEmpty(t *testing.T) {
	t.Parallel()
	composed := ComposeNormalizers()
	got := string(composed([]byte("unchanged")))
	if got != "unchanged" {
		t.Errorf("got %q, want %q", got, "unchanged")
	}
}

func TestRunDiffTestsMatchingOutput(t *testing.T) {
	t.Parallel()
	bin := buildMockScript(t, `echo hello`)
	RunDiffTests(t, bin, bin, []DiffTest{{Name: "match"}})
}

func TestRunDiffTestsMatchingWithStdin(t *testing.T) {
	t.Parallel()
	bin := buildMockScript(t, `cat`)
	RunDiffTests(t, bin, bin, []DiffTest{
		{Name: "stdin_echo", Stdin: []byte("input data\n")},
	})
}

func TestRunDiffTestsDivergentStdout(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_SUBPROCESS_DIVERGENT_STDOUT") == "1" {
		bin1 := buildMockScript(t, `echo hello`)
		bin2 := buildMockScript(t, `echo world`)
		RunDiffTests(t, bin1, bin2, []DiffTest{{Name: "diverge"}})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunDiffTestsDivergentStdout$", "-test.v")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS_DIVERGENT_STDOUT=1")
	output, _ := cmd.CombinedOutput()

	if !strings.Contains(string(output), "divergence detected") {
		t.Errorf("expected divergence message in output:\n%s", output)
	}
}

func TestRunDiffTestsDivergentExitCode(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_SUBPROCESS_DIVERGENT_EXIT") == "1" {
		bin1 := buildMockScript(t, `exit 0`)
		bin2 := buildMockScript(t, `exit 1`)
		RunDiffTests(t, bin1, bin2, []DiffTest{{Name: "exit_diff"}})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunDiffTestsDivergentExitCode$", "-test.v")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS_DIVERGENT_EXIT=1")
	output, _ := cmd.CombinedOutput()

	if !strings.Contains(string(output), "divergence detected") {
		t.Errorf("expected divergence message in output:\n%s", output)
	}
	if !strings.Contains(string(output), "ref exit:") {
		t.Errorf("expected exit code info in output:\n%s", output)
	}
}

func TestRunDiffTestsEnvDefaultLCALL(t *testing.T) {
	t.Parallel()
	// Script prints the LC_ALL env var; both binaries are the same.
	bin := buildMockScript(t, `printf "%s" "$LC_ALL"`)
	// If LC_ALL=C is set correctly, both binaries print "C" and match.
	RunDiffTests(t, bin, bin, []DiffTest{{Name: "lc_all_default"}})
}

func TestRunDiffTestsEnvOverride(t *testing.T) {
	t.Parallel()
	bin := buildMockScript(t, `printf "%s" "$LC_ALL"`)
	RunDiffTests(t, bin, bin, []DiffTest{
		{Name: "lc_all_override", Env: []string{"LC_ALL=en_US.UTF-8"}},
	})
}

func TestRunDiffTestsWorkDirDefault(t *testing.T) {
	t.Parallel()
	// Both binaries print their working directory; they should match.
	bin := buildMockScript(t, `pwd`)
	RunDiffTests(t, bin, bin, []DiffTest{{Name: "workdir_default"}})
}

func TestRunDiffTestsWorkDirExplicit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bin := buildMockScript(t, `pwd`)
	RunDiffTests(t, bin, bin, []DiffTest{
		{Name: "workdir_explicit", WorkDir: dir},
	})
}

func TestRunDiffTestsWithNormalizer(t *testing.T) {
	t.Parallel()
	// Two binaries produce different timestamps but same text after normalization.
	bin1 := buildMockScript(t, `printf "Feb 19 12:34:56 event"`)
	bin2 := buildMockScript(t, `printf "Mar 01 23:59:59 event"`)
	RunDiffTests(t, bin1, bin2, []DiffTest{
		{Name: "normalized", Normalize: []NormalizeFunc{TimestampNormalizer}},
	})
}

func TestRunDiffTestsExpectedFilesMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bin := buildMockScript(t, `printf "file content" > "$PWD/output.txt"`)
	RunDiffTests(t, bin, bin, []DiffTest{
		{
			Name:          "expected_files",
			WorkDir:       dir,
			ExpectedFiles: map[string][]byte{"output.txt": []byte("file content")},
		},
	})
}

func TestRunDiffTestsExpectedFilesDiverge(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_SUBPROCESS_FILE_DIVERGE") == "1" {
		dir := t.TempDir()
		bin := buildMockScript(t, `printf "actual" > "$PWD/out.txt"`)
		RunDiffTests(t, bin, bin, []DiffTest{
			{
				Name:          "file_diverge",
				WorkDir:       dir,
				ExpectedFiles: map[string][]byte{"out.txt": []byte("expected")},
			},
		})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunDiffTestsExpectedFilesDiverge$", "-test.v")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS_FILE_DIVERGE=1")
	output, _ := cmd.CombinedOutput()

	if !strings.Contains(string(output), "file") && !strings.Contains(string(output), "divergence") {
		t.Errorf("expected file divergence message:\n%s", output)
	}
}

func TestRunDiffTestsTimeout(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_SUBPROCESS_TIMEOUT") == "1" {
		// Set a very short timeout for this test.
		defaultTimeout = 100 * time.Millisecond
		bin := buildMockScript(t, `sleep 30`)
		RunDiffTests(t, bin, bin, []DiffTest{{Name: "timeout"}})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunDiffTestsTimeout$", "-test.v")
	cmd.Env = append(os.Environ(), "TEST_SUBPROCESS_TIMEOUT=1")
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected test to fail due to timeout")
	}
	if !strings.Contains(string(output), "timed out") {
		t.Errorf("expected timeout message in output:\n%s", output)
	}
}

func TestRunDiffTestsZeroValue(t *testing.T) {
	t.Parallel()
	// Zero-value DiffTest: no args, nil stdin, nil env, empty workdir, exit 0.
	bin := buildMockScript(t, `true`)
	RunDiffTests(t, bin, bin, []DiffTest{{Name: "zero"}})
}

func TestBuildBinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"),
		[]byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module testmod\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	binPath := BuildBinary(t, dir)

	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("binary not found: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("binary is not executable")
	}
}
