// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for runner.go: buildEnv, setEnvVar, formatDivergence, truncateBytes,
// compareExpectedFiles.
// Implements: prd001-testutils (R2.6, R3.5, R5.1, R5.2)
package testutils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildEnv(t *testing.T) {
	tests := []struct {
		name    string
		testEnv []string
		check   func(t *testing.T, env []string)
	}{
		{
			name:    "default sets LC_ALL=C",
			testEnv: nil,
			check: func(t *testing.T, env []string) {
				t.Helper()
				assertEnvContains(t, env, "LC_ALL", "C")
			},
		},
		{
			name:    "caller variable is merged",
			testEnv: []string{"MY_CUSTOM_VAR=hello"},
			check: func(t *testing.T, env []string) {
				t.Helper()
				assertEnvContains(t, env, "MY_CUSTOM_VAR", "hello")
				// LC_ALL=C should still be present
				assertEnvContains(t, env, "LC_ALL", "C")
			},
		},
		{
			name:    "caller overrides LC_ALL",
			testEnv: []string{"LC_ALL=en_US.UTF-8"},
			check: func(t *testing.T, env []string) {
				t.Helper()
				assertEnvContains(t, env, "LC_ALL", "en_US.UTF-8")
				// Must not have duplicate LC_ALL entries
				count := countEnvKey(env, "LC_ALL")
				if count != 1 {
					t.Fatalf("expected exactly 1 LC_ALL entry, got %d", count)
				}
			},
		},
		{
			name:    "override replaces rather than duplicates",
			testEnv: []string{"PATH=/custom/path"},
			check: func(t *testing.T, env []string) {
				t.Helper()
				assertEnvContains(t, env, "PATH", "/custom/path")
				count := countEnvKey(env, "PATH")
				if count != 1 {
					t.Fatalf("expected exactly 1 PATH entry, got %d", count)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := buildEnv(tc.testEnv)
			tc.check(t, env)
		})
	}
}

func TestSetEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		initial  []string
		key      string
		value    string
		wantKey  string
		wantVal  string
		wantLen  int
	}{
		{
			name:    "add new variable to empty slice",
			initial: []string{},
			key:     "FOO",
			value:   "bar",
			wantKey: "FOO",
			wantVal: "bar",
			wantLen: 1,
		},
		{
			name:    "add new variable to existing slice",
			initial: []string{"A=1", "B=2"},
			key:     "C",
			value:   "3",
			wantKey: "C",
			wantVal: "3",
			wantLen: 3,
		},
		{
			name:    "override existing variable",
			initial: []string{"A=1", "B=2", "C=3"},
			key:     "B",
			value:   "replaced",
			wantKey: "B",
			wantVal: "replaced",
			wantLen: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := setEnvVar(tc.initial, tc.key, tc.value)
			if len(result) != tc.wantLen {
				t.Fatalf("expected length %d, got %d", tc.wantLen, len(result))
			}
			assertEnvContains(t, result, tc.wantKey, tc.wantVal)
		})
	}
}

func TestFormatDivergence(t *testing.T) {
	tc := DiffTest{
		Name:  "test-case",
		Args:  []string{"-n", "file.txt"},
		Stdin: []byte("input data"),
	}
	refStdout := []byte("ref output")
	goStdout := []byte("go output")
	refStderr := []byte("ref error")
	goStderr := []byte("go error")
	refExit := 0
	goExit := 1

	msg := formatDivergence(tc, refStdout, goStdout, refStderr, goStderr, refExit, goExit)

	// Verify all required fields are present in the divergence report (prd001-testutils R3.5)
	requiredFields := []struct {
		label string
		value string
	}{
		{"args", "-n"},
		{"args", "file.txt"},
		{"stdin", "input data"},
		{"ref stdout", "ref output"},
		{"go stdout", "go output"},  // labeled "go  stdout" in format string
		{"ref stderr", "ref error"},
		{"go stderr", "go error"},   // labeled "go  stderr" in format string
		{"ref exit", "0"},
		{"go exit", "1"},            // labeled "go  exit" in format string
	}
	for _, f := range requiredFields {
		if !strings.Contains(msg, f.value) {
			t.Fatalf("divergence message missing %s value %q\nmessage: %s", f.label, f.value, msg)
		}
	}
}

func TestFormatDivergenceNilStdin(t *testing.T) {
	tc := DiffTest{
		Name: "nil-stdin",
		Args: []string{},
	}
	msg := formatDivergence(tc, []byte("a"), []byte("b"), nil, nil, 0, 0)
	if !strings.Contains(msg, "<nil>") {
		t.Fatalf("expected <nil> for nil stdin, got: %s", msg)
	}
}

func TestTruncateBytes(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		maxLen int
		want   string
	}{
		{
			name:   "nil input returns <nil>",
			input:  nil,
			maxLen: 256,
			want:   "<nil>",
		},
		{
			name:   "short input returns full content",
			input:  []byte("hello"),
			maxLen: 256,
			want:   `"hello"`,
		},
		{
			name:   "exact length returns full content",
			input:  []byte("abc"),
			maxLen: 3,
			want:   `"abc"`,
		},
		{
			name:   "long input is truncated with byte count",
			input:  []byte("abcdefghij"),
			maxLen: 5,
			want:   "", // checked below
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateBytes(tc.input, tc.maxLen)
			if tc.name == "long input is truncated with byte count" {
				// Must contain "truncated" and the total byte count
				if !strings.Contains(got, "truncated") {
					t.Fatalf("expected 'truncated' in output, got: %s", got)
				}
				if !strings.Contains(got, "10 bytes total") {
					t.Fatalf("expected '10 bytes total' in output, got: %s", got)
				}
				// Must not contain the full input
				if strings.Contains(got, "abcdefghij") {
					t.Fatalf("expected truncated output, got full content: %s", got)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestCompareExpectedFiles(t *testing.T) {
	t.Run("nil expected files does nothing", func(t *testing.T) {
		// Should not panic or fail
		compareExpectedFiles(t, nil, t.TempDir())
	})

	t.Run("matching file content passes", func(t *testing.T) {
		dir := t.TempDir()
		content := []byte("expected content\n")
		if err := os.WriteFile(filepath.Join(dir, "output.txt"), content, 0644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		// This should pass without calling t.Fatal
		compareExpectedFiles(t, map[string][]byte{
			"output.txt": content,
		}, dir)
	})

	t.Run("mismatched file content fails", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "output.txt"), []byte("actual"), 0644); err != nil {
			t.Fatalf("setup: %v", err)
		}

		mockT := &testing.T{}
		done := make(chan struct{})
		go func() {
			defer func() { close(done) }()
			compareExpectedFiles(mockT, map[string][]byte{
				"output.txt": []byte("expected"),
			}, dir)
		}()
		<-done
		if !mockT.Failed() {
			t.Fatal("expected compareExpectedFiles to fail on content mismatch")
		}
	})

	t.Run("missing file fails", func(t *testing.T) {
		dir := t.TempDir()

		mockT := &testing.T{}
		done := make(chan struct{})
		go func() {
			defer func() { close(done) }()
			compareExpectedFiles(mockT, map[string][]byte{
				"nonexistent.txt": []byte("content"),
			}, dir)
		}()
		<-done
		if !mockT.Failed() {
			t.Fatal("expected compareExpectedFiles to fail on missing file")
		}
	})
}

// assertEnvContains checks that the env slice contains key=value.
func assertEnvContains(t *testing.T, env []string, key, value string) {
	t.Helper()
	target := key + "=" + value
	for _, e := range env {
		if e == target {
			return
		}
	}
	t.Fatalf("expected %s=%s in env, not found", key, value)
}

// countEnvKey counts how many entries in env start with key=.
func countEnvKey(env []string, key string) int {
	prefix := key + "="
	count := 0
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			count++
		}
	}
	return count
}
