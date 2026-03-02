// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for the differential testing harness.
//
// Implements: prd001-testutils R2, R3, R4, R5
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

// helperEchoSource is the Go source for a helper binary that echoes stdin to
// stdout and exits with the code given as its first argument (default 0).
const helperEchoSource = `package main

import (
	"io"
	"os"
	"strconv"
)

func main() {
	exitCode := 0
	if len(os.Args) > 1 {
		exitCode, _ = strconv.Atoi(os.Args[1])
	}
	io.Copy(os.Stdout, os.Stdin)
	os.Exit(exitCode)
}
`

// helperFixedSource is the Go source for a helper binary that writes fixed
// output to stdout and stderr, then exits with code 42.
const helperFixedSource = `package main

import "os"

func main() {
	os.Stdout.WriteString("fixed-stdout\n")
	os.Stderr.WriteString("fixed-stderr\n")
	os.Exit(42)
}
`

// envSubprocess signals that the test binary is running in a subprocess for
// expected-failure verification.
const envSubprocess = "TESTUTILS_SUBPROCESS"

var (
	echoHelperPath  string
	fixedHelperPath string
)

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "testutils-helpers-*")
	if err != nil {
		panic("creating temp dir for helpers: " + err.Error())
	}

	echoHelperPath, err = buildHelper(tmpDir, "echo-helper", helperEchoSource)
	if err != nil {
		os.RemoveAll(tmpDir) // best-effort cleanup
		panic("building echo helper: " + err.Error())
	}

	fixedHelperPath, err = buildHelper(tmpDir, "fixed-helper", helperFixedSource)
	if err != nil {
		os.RemoveAll(tmpDir) // best-effort cleanup
		panic("building fixed helper: " + err.Error())
	}

	code := m.Run()
	os.RemoveAll(tmpDir) // best-effort cleanup
	os.Exit(code)
}

// buildHelper compiles a single-file Go main program and returns the binary path.
func buildHelper(parentDir, name, source string) (string, error) {
	srcDir := filepath.Join(parentDir, "src-"+name)
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return "", err
	}

	goMod := "module " + name + "\n\ngo 1.25\n"
	if err := os.WriteFile(filepath.Join(srcDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte(source), 0o644); err != nil {
		return "", err
	}

	binPath := filepath.Join(parentDir, name)
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = srcDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build %s: %w\n%s", name, err, out)
	}
	return binPath, nil
}

// envContains reports whether env contains the exact entry string.
func envContains(env []string, entry string) bool {
	for _, e := range env {
		if e == entry {
			return true
		}
	}
	return false
}

// --- buildEnv tests (prd001-testutils R2.6, R1.3) ---

func TestBuildEnv(t *testing.T) {
	tests := []struct {
		name        string
		testEnv     []string
		mustHave    []string
		mustNotHave []string
	}{
		{
			name:     "nil-env-sets-LC_ALL",
			testEnv:  nil,
			mustHave: []string{"LC_ALL=C"},
		},
		{
			name:     "custom-env-merged-with-LC_ALL",
			testEnv:  []string{"CUSTOM_VAR=value"},
			mustHave: []string{"LC_ALL=C", "CUSTOM_VAR=value"},
		},
		{
			name:        "LC_ALL-override-replaces-default",
			testEnv:     []string{"LC_ALL=en_US.UTF-8"},
			mustHave:    []string{"LC_ALL=en_US.UTF-8"},
			mustNotHave: []string{"LC_ALL=C"},
		},
		{
			name:     "new-key-appended",
			testEnv:  []string{"BRAND_NEW_KEY=123"},
			mustHave: []string{"LC_ALL=C", "BRAND_NEW_KEY=123"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := buildEnv(tc.testEnv)
			for _, want := range tc.mustHave {
				if !envContains(env, want) {
					t.Errorf("expected env to contain %q", want)
				}
			}
			for _, reject := range tc.mustNotHave {
				if envContains(env, reject) {
					t.Errorf("expected env NOT to contain %q", reject)
				}
			}
		})
	}
}

// --- applyNormalizers tests (prd001-testutils R4.1, R4.3) ---

func TestApplyNormalizers(t *testing.T) {
	upper := func(data []byte) []byte { return bytes.ToUpper(data) }
	addSuffix := func(data []byte) []byte { return append(data, "-suffix"...) }

	tests := []struct {
		name   string
		input  []byte
		fns    []NormalizeFunc
		expect []byte
	}{
		{
			name:   "nil-normalizers-unchanged",
			input:  []byte("hello"),
			fns:    nil,
			expect: []byte("hello"),
		},
		{
			name:   "empty-normalizers-unchanged",
			input:  []byte("hello"),
			fns:    []NormalizeFunc{},
			expect: []byte("hello"),
		},
		{
			name:   "single-normalizer-applied",
			input:  []byte("hello"),
			fns:    []NormalizeFunc{upper},
			expect: []byte("HELLO"),
		},
		{
			name:   "multiple-normalizers-applied-in-order",
			input:  []byte("hello"),
			fns:    []NormalizeFunc{upper, addSuffix},
			expect: []byte("HELLO-suffix"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := applyNormalizers(tc.input, tc.fns)
			if !bytes.Equal(got, tc.expect) {
				t.Errorf("got %q, want %q", got, tc.expect)
			}
		})
	}
}

// --- TimestampNormalizer tests (prd001-testutils R4.2) ---

func TestTimestampNormalizer(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "syslog-format",
			input:  "Feb 19 12:34:56 some log message",
			expect: timestampPlaceholder + " some log message",
		},
		{
			name:   "iso-format",
			input:  "2026-02-19 12:34:56 event happened",
			expect: timestampPlaceholder + " event happened",
		},
		{
			name:   "no-timestamps-unchanged",
			input:  "plain text with no timestamps",
			expect: "plain text with no timestamps",
		},
		{
			name:   "multiple-syslog-timestamps",
			input:  "Jan  1 00:00:00 start\nDec 31 23:59:59 end",
			expect: timestampPlaceholder + " start\n" + timestampPlaceholder + " end",
		},
		{
			name:   "mixed-syslog-and-iso",
			input:  "Feb 19 12:34:56 and 2026-02-19 12:34:56",
			expect: timestampPlaceholder + " and " + timestampPlaceholder,
		},
		{
			name:   "syslog-single-digit-day",
			input:  "Mar  5 08:15:30 event",
			expect: timestampPlaceholder + " event",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TimestampNormalizer([]byte(tc.input))
			if string(got) != tc.expect {
				t.Errorf("got %q, want %q", string(got), tc.expect)
			}
		})
	}
}

// --- ComposeNormalizers tests (prd001-testutils R4.4) ---

func TestComposeNormalizers(t *testing.T) {
	trimSpace := func(data []byte) []byte { return bytes.TrimSpace(data) }
	upper := func(data []byte) []byte { return bytes.ToUpper(data) }

	composed := ComposeNormalizers(trimSpace, upper)
	got := composed([]byte("  hello  "))
	want := []byte("HELLO")
	if !bytes.Equal(got, want) {
		t.Errorf("ComposeNormalizers: got %q, want %q", got, want)
	}
}

// --- truncateStdin tests ---

func TestTruncateStdin(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		expect string
	}{
		{
			name:   "nil-stdin",
			input:  nil,
			expect: "<nil>",
		},
		{
			name:   "short-stdin",
			input:  []byte("hello"),
			expect: fmt.Sprintf("%q", []byte("hello")),
		},
		{
			name:   "at-limit",
			input:  bytes.Repeat([]byte("a"), stdinMaxDisplay),
			expect: fmt.Sprintf("%q", bytes.Repeat([]byte("a"), stdinMaxDisplay)),
		},
		{
			name:   "over-limit-truncated",
			input:  bytes.Repeat([]byte("a"), stdinMaxDisplay+10),
			expect: fmt.Sprintf("%q... (%d bytes total)", bytes.Repeat([]byte("a"), stdinMaxDisplay), stdinMaxDisplay+10),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateStdin(tc.input)
			if got != tc.expect {
				t.Errorf("got %s, want %s", got, tc.expect)
			}
		})
	}
}

// --- RunDiffTests matching test (prd001-testutils R2.1, R3.2, R3.3, R3.4) ---

func TestRunDiffTests_Matching(t *testing.T) {
	tests := []DiffTest{
		{
			Name:  "identical-echo-with-stdin",
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "empty-stdin",
			Stdin: []byte{},
		},
		{
			Name:  "nil-stdin",
			Stdin: nil,
		},
	}

	RunDiffTests(t, echoHelperPath, echoHelperPath, tests)
}

// --- RunDiffTests diverging test (prd001-testutils R3.2, R3.3, R3.4, R3.5) ---

func TestRunDiffTests_Diverging(t *testing.T) {
	if os.Getenv(envSubprocess) == "diverge" {
		tests := []DiffTest{
			{
				Name:  "stdout-divergence",
				Stdin: []byte("echo-input\n"),
			},
		}
		RunDiffTests(t, fixedHelperPath, echoHelperPath, tests)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunDiffTests_Diverging$", "-test.v")
	cmd.Env = append(os.Environ(), envSubprocess+"=diverge")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected test to fail due to divergence, but it passed")
	}

	output := string(out)
	requiredFields := []string{
		"divergence detected",
		"args:",
		"stdin:",
		"ref stdout:",
		"go  stdout:",
		"ref stderr:",
		"go  stderr:",
		"ref exit:",
		"go  exit:",
	}
	for _, field := range requiredFields {
		if !strings.Contains(output, field) {
			t.Errorf("expected %q in divergence output, got:\n%s", field, output)
		}
	}
}

// --- checkExpectedFiles tests (prd001-testutils R5.1, R5.2) ---

func TestCheckExpectedFiles_Match(t *testing.T) {
	tmpDir := t.TempDir()
	content := []byte("expected content\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "output.txt"), content, 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tc := DiffTest{
		ExpectedFiles: map[string][]byte{
			"output.txt": content,
		},
	}
	checkExpectedFiles(t, tc, tmpDir)
}

func TestCheckExpectedFiles_Mismatch(t *testing.T) {
	if os.Getenv(envSubprocess) == "file-mismatch" {
		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "output.txt"), []byte("actual\n"), 0o644); err != nil {
			t.Fatalf("writing test file: %v", err)
		}
		tc := DiffTest{
			ExpectedFiles: map[string][]byte{
				"output.txt": []byte("expected\n"),
			},
		}
		checkExpectedFiles(t, tc, tmpDir)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestCheckExpectedFiles_Mismatch$", "-test.v")
	cmd.Env = append(os.Environ(), envSubprocess+"=file-mismatch")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected test to fail due to file content mismatch, but it passed")
	}
	if !strings.Contains(string(out), "file content divergence") {
		t.Fatalf("expected 'file content divergence' in output, got:\n%s", out)
	}
}

func TestCheckExpectedFiles_Missing(t *testing.T) {
	if os.Getenv(envSubprocess) == "file-missing" {
		tmpDir := t.TempDir()
		tc := DiffTest{
			ExpectedFiles: map[string][]byte{
				"nonexistent.txt": []byte("anything\n"),
			},
		}
		checkExpectedFiles(t, tc, tmpDir)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestCheckExpectedFiles_Missing$", "-test.v")
	cmd.Env = append(os.Environ(), envSubprocess+"=file-missing")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected test to fail due to missing file, but it passed")
	}
	if !strings.Contains(string(out), "expected file") {
		t.Fatalf("expected 'expected file' in output, got:\n%s", out)
	}
}
