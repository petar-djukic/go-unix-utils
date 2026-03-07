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

func TestComposeNormalizers_Empty(t *testing.T) {
	t.Parallel()
	// R4.4: composing zero normalizers returns input unchanged.
	fn := ComposeNormalizers()
	input := []byte("hello world")
	got := fn(input)
	if !bytes.Equal(got, input) {
		t.Errorf("ComposeNormalizers() = %q, want %q", got, input)
	}
}

func TestComposeNormalizers_Single(t *testing.T) {
	t.Parallel()
	upper := func(b []byte) []byte { return bytes.ToUpper(b) }
	fn := ComposeNormalizers(upper)
	got := fn([]byte("hello"))
	if !bytes.Equal(got, []byte("HELLO")) {
		t.Errorf("ComposeNormalizers(upper) = %q, want %q", got, "HELLO")
	}
}

func TestComposeNormalizers_Order(t *testing.T) {
	t.Parallel()
	// R4.4: functions are applied left to right.
	appendA := func(b []byte) []byte { return append(b, 'A') }
	appendB := func(b []byte) []byte { return append(b, 'B') }
	fn := ComposeNormalizers(appendA, appendB)
	got := fn([]byte("x"))
	want := []byte("xAB")
	if !bytes.Equal(got, want) {
		t.Errorf("ComposeNormalizers(appendA, appendB) = %q, want %q", got, want)
	}
}

func TestTimestampNormalizer_MonDDTime(t *testing.T) {
	t.Parallel()
	// R4.2: "Mon DD HH:MM:SS" pattern.
	input := []byte("event at Feb 19 12:34:56 done")
	got := TimestampNormalizer(input)
	want := []byte("event at <TIMESTAMP> done")
	if !bytes.Equal(got, want) {
		t.Errorf("TimestampNormalizer(%q) = %q, want %q", input, got, want)
	}
}

func TestTimestampNormalizer_YYYYMMDD(t *testing.T) {
	t.Parallel()
	// R4.2: "YYYY-MM-DD HH:MM:SS" pattern matches the full datetime.
	input := []byte("log 2026-02-19 08:15:30 end")
	got := TimestampNormalizer(input)
	want := []byte("log <TIMESTAMP> end")
	if !bytes.Equal(got, want) {
		t.Errorf("TimestampNormalizer(%q) = %q, want %q", input, got, want)
	}
}

func TestTimestampNormalizer_TimeOnly(t *testing.T) {
	t.Parallel()
	// R4.2: "HH:MM:SS" standalone pattern.
	input := []byte("at 23:59:59 sharp")
	got := TimestampNormalizer(input)
	want := []byte("at <TIMESTAMP> sharp")
	if !bytes.Equal(got, want) {
		t.Errorf("TimestampNormalizer(%q) = %q, want %q", input, got, want)
	}
}

func TestTimestampNormalizer_NoTimestamp(t *testing.T) {
	t.Parallel()
	input := []byte("no timestamps here")
	got := TimestampNormalizer(input)
	if !bytes.Equal(got, input) {
		t.Errorf("TimestampNormalizer(%q) = %q, want unchanged", input, got)
	}
}

func TestTimestampNormalizer_Multiple(t *testing.T) {
	t.Parallel()
	input := []byte("start 01:02:03 mid 04:05:06 end")
	got := TimestampNormalizer(input)
	want := []byte("start <TIMESTAMP> mid <TIMESTAMP> end")
	if !bytes.Equal(got, want) {
		t.Errorf("TimestampNormalizer(%q) = %q, want %q", input, got, want)
	}
}

func TestBuildBinary_Version(t *testing.T) {
	t.Parallel()
	// R4 / AC4: BuildBinary compiles a Go package and returns its path.
	binPath := BuildBinary(t, "../../cmd/version")
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("BuildBinary returned path that does not exist: %v", err)
	}

	// Verify the binary runs.
	out, err := exec.Command(binPath).Output()
	if err != nil {
		t.Fatalf("running built binary: %v", err)
	}
	if !bytes.Contains(out, []byte("dev")) {
		t.Errorf("version binary output = %q, want to contain \"dev\"", out)
	}
}

func TestRunDiffTests_MatchingOutput(t *testing.T) {
	t.Parallel()
	// R2.1, R2.4, R3.2: both binaries produce identical output, test passes.
	echoBin := findEcho(t)
	tests := []DiffTest{
		{Name: "hello", Args: []string{"hello"}, ExitCode: 0},
	}
	RunDiffTests(t, echoBin, echoBin, tests)
}

func TestRunDiffTests_ZeroValue(t *testing.T) {
	t.Parallel()
	// R1.1: DiffTest zero value is a valid test case.
	echoBin := findEcho(t)
	tests := []DiffTest{
		{Name: "zero"},
	}
	RunDiffTests(t, echoBin, echoBin, tests)
}

func TestRunDiffTests_WithStdin(t *testing.T) {
	t.Parallel()
	// R1.2: Stdin is passed to both binaries.
	catBin := findCat(t)
	tests := []DiffTest{
		{Name: "stdin-echo", Stdin: []byte("hello\n"), ExitCode: 0},
	}
	RunDiffTests(t, catBin, catBin, tests)
}

func TestRunDiffTests_WithNormalizer(t *testing.T) {
	t.Parallel()
	// R4.1, R4.3: normalizers are applied before comparison.
	echoBin := findEcho(t)
	upper := func(b []byte) []byte { return bytes.ToUpper(b) }
	tests := []DiffTest{
		{
			Name:      "normalized",
			Args:      []string{"hello"},
			ExitCode:  0,
			Normalize: []NormalizeFunc{upper},
		},
	}
	RunDiffTests(t, echoBin, echoBin, tests)
}

func TestRunDiffTests_ExpectedFiles(t *testing.T) {
	t.Parallel()
	// R5.1, R5.2: ExpectedFiles verified after execution.
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "out.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	echoBin := findEcho(t)
	tests := []DiffTest{
		{
			Name:          "file-check",
			Args:          []string{"ok"},
			ExitCode:      0,
			WorkDir:       tmp,
			ExpectedFiles: map[string][]byte{"out.txt": []byte("content")},
		},
	}
	RunDiffTests(t, echoBin, echoBin, tests)
}

func TestBuildEnv_DefaultLC(t *testing.T) {
	t.Parallel()
	// R2.6: LC_ALL=C is set by default.
	env := buildEnv(nil)
	found := false
	for _, e := range env {
		if e == "LC_ALL=C" {
			found = true
			break
		}
	}
	if !found {
		t.Error("buildEnv(nil) does not contain LC_ALL=C")
	}
}

func TestBuildEnv_OverrideLC(t *testing.T) {
	t.Parallel()
	// R2.6, R1.3: DiffTest.Env overrides LC_ALL.
	env := buildEnv([]string{"LC_ALL=en_US.UTF-8"})
	for _, e := range env {
		if e == "LC_ALL=C" {
			t.Error("buildEnv with LC_ALL override still contains LC_ALL=C")
		}
	}
	found := false
	for _, e := range env {
		if e == "LC_ALL=en_US.UTF-8" {
			found = true
			break
		}
	}
	if !found {
		t.Error("buildEnv did not apply LC_ALL=en_US.UTF-8 override")
	}
}

func TestSetEnvVar_New(t *testing.T) {
	t.Parallel()
	env := []string{"A=1"}
	env = setEnvVar(env, "B", "2")
	if len(env) != 2 || env[1] != "B=2" {
		t.Errorf("setEnvVar new key: got %v", env)
	}
}

func TestSetEnvVar_Override(t *testing.T) {
	t.Parallel()
	env := []string{"A=1", "B=2"}
	env = setEnvVar(env, "A", "99")
	if env[0] != "A=99" {
		t.Errorf("setEnvVar override: got %v", env)
	}
	if len(env) != 2 {
		t.Errorf("setEnvVar override changed length: got %d", len(env))
	}
}

func TestApplyNormalizers_Nil(t *testing.T) {
	t.Parallel()
	input := []byte("unchanged")
	got := applyNormalizers(input, nil)
	if !bytes.Equal(got, input) {
		t.Errorf("applyNormalizers(nil) = %q, want %q", got, input)
	}
}

func TestApplyNormalizers_Chain(t *testing.T) {
	t.Parallel()
	upper := func(b []byte) []byte { return bytes.ToUpper(b) }
	trim := func(b []byte) []byte { return bytes.TrimSpace(b) }
	got := applyNormalizers([]byte("  hello  "), []NormalizeFunc{trim, upper})
	want := []byte("HELLO")
	if !bytes.Equal(got, want) {
		t.Errorf("applyNormalizers chain = %q, want %q", got, want)
	}
}

func TestFormatDivergence_ContainsFields(t *testing.T) {
	t.Parallel()
	tc := DiffTest{
		Name:  "test",
		Args:  []string{"-n", "file"},
		Stdin: []byte("input data"),
	}
	msg := formatDivergence(tc, []byte("ref"), []byte("go"), []byte("refE"), []byte("goE"), 0, 1)
	for _, want := range []string{"Args:", "Stdin:", "Reference stdout:", "Go stdout:", "Reference stderr:", "Go stderr:", "Reference exit:", "Go exit:"} {
		if !strings.Contains(msg, want) {
			t.Errorf("formatDivergence missing %q in:\n%s", want, msg)
		}
	}
}

func TestFormatDivergence_TruncatesStdin(t *testing.T) {
	t.Parallel()
	longStdin := bytes.Repeat([]byte("x"), 300)
	tc := DiffTest{Name: "trunc", Stdin: longStdin}
	msg := formatDivergence(tc, nil, nil, nil, nil, 0, 0)
	if !strings.Contains(msg, "truncated") {
		t.Errorf("formatDivergence did not truncate long stdin in:\n%s", msg)
	}
}

// findEcho returns the path to /bin/echo, skipping if not found.
func findEcho(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("echo")
	if err != nil {
		t.Skipf("echo not in PATH: %v", err)
	}
	return path
}

// findCat returns the path to cat, skipping if not found.
func findCat(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("cat")
	if err != nil {
		t.Skipf("cat not in PATH: %v", err)
	}
	return path
}
