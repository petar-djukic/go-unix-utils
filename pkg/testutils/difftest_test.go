// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestComposeNormalizersChaining(t *testing.T) {
	t.Parallel()

	// AC3: output of first normalizer is input to second.
	upper := func(data []byte) []byte {
		return bytes.ReplaceAll(data, []byte("hello"), []byte("HELLO"))
	}
	exclaim := func(data []byte) []byte {
		return bytes.ReplaceAll(data, []byte("HELLO"), []byte("HELLO!"))
	}

	composed := ComposeNormalizers(upper, exclaim)
	got := composed([]byte("hello world"))
	want := []byte("HELLO! world")
	if !bytes.Equal(got, want) {
		t.Errorf("ComposeNormalizers: got %q, want %q", got, want)
	}
}

func TestComposeNormalizersEmpty(t *testing.T) {
	t.Parallel()

	// Composing zero normalizers returns a function that passes through.
	composed := ComposeNormalizers()
	input := []byte("unchanged")
	got := composed(input)
	if !bytes.Equal(got, input) {
		t.Errorf("ComposeNormalizers(): got %q, want %q", got, input)
	}
}

func TestComposeNormalizersSingle(t *testing.T) {
	t.Parallel()

	fn := func(data []byte) []byte {
		return bytes.ReplaceAll(data, []byte("a"), []byte("b"))
	}
	composed := ComposeNormalizers(fn)
	got := composed([]byte("aaa"))
	want := []byte("bbb")
	if !bytes.Equal(got, want) {
		t.Errorf("ComposeNormalizers(single): got %q, want %q", got, want)
	}
}

func TestTimestampNormalizerISO(t *testing.T) {
	t.Parallel()

	// AC4: ISO 8601 timestamps are replaced.
	input := []byte("event at 2026-02-19 12:34:56 happened")
	got := TimestampNormalizer(input)
	if bytes.Contains(got, []byte("2026-02-19 12:34:56")) {
		t.Errorf("TimestampNormalizer did not replace ISO timestamp: %q", got)
	}
	if !bytes.Contains(got, []byte(timestampPlaceholder)) {
		t.Errorf("TimestampNormalizer missing placeholder in: %q", got)
	}
}

func TestTimestampNormalizerISO_T(t *testing.T) {
	t.Parallel()

	input := []byte("2026-02-19T12:34:56")
	got := TimestampNormalizer(input)
	if bytes.Contains(got, []byte("2026-02-19T12:34:56")) {
		t.Errorf("TimestampNormalizer did not replace ISO-T timestamp: %q", got)
	}
}

func TestTimestampNormalizerCtime(t *testing.T) {
	t.Parallel()

	// AC4: ctime-style timestamps are replaced.
	input := []byte("log Feb 19 12:34:56 entry")
	got := TimestampNormalizer(input)
	if bytes.Contains(got, []byte("Feb 19 12:34:56")) {
		t.Errorf("TimestampNormalizer did not replace ctime timestamp: %q", got)
	}
}

func TestTimestampNormalizerTimeOnly(t *testing.T) {
	t.Parallel()

	input := []byte("at 12:34:56 done")
	got := TimestampNormalizer(input)
	if bytes.Contains(got, []byte("12:34:56")) {
		t.Errorf("TimestampNormalizer did not replace time-only: %q", got)
	}
}

func TestTimestampNormalizerNoTimestamp(t *testing.T) {
	t.Parallel()

	input := []byte("no timestamps here")
	got := TimestampNormalizer(input)
	if !bytes.Equal(got, input) {
		t.Errorf("TimestampNormalizer modified non-timestamp input: got %q, want %q", got, input)
	}
}

func TestComposeSliceNil(t *testing.T) {
	t.Parallel()

	got := composeSlice(nil)
	if got != nil {
		t.Error("composeSlice(nil) should return nil")
	}
}

func TestComposeSliceEmpty(t *testing.T) {
	t.Parallel()

	got := composeSlice([]NormalizeFunc{})
	if got != nil {
		t.Error("composeSlice(empty) should return nil")
	}
}

func TestBuildBinary(t *testing.T) {
	t.Parallel()

	// AC5: BuildBinary compiles a Go package and returns a usable binary path.
	// Use a known-good package from the project: the echo-like binary is simple,
	// but we can use the testutils package itself is not main. Use cmd/cat as
	// it exists in the project.
	binPath := BuildBinary(t, ".")
	if binPath == "" {
		t.Fatal("BuildBinary returned empty path")
	}
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("BuildBinary output does not exist: %v", err)
	}
}

func TestRunDiffTestsIdenticalBinaries(t *testing.T) {
	t.Parallel()

	// AC2: RunDiffTests compares stdout, stderr, and exit code.
	// Use the same binary (echo) as both go and ref to ensure they match.
	echoBin, err := exec.LookPath("echo")
	if err != nil {
		t.Skip("echo not in PATH")
	}

	tests := []DiffTest{
		{
			Name: "identical_echo",
			Args: []string{"hello", "world"},
		},
	}
	RunDiffTests(t, echoBin, echoBin, tests)
}

func TestRunDiffTestsWithStdin(t *testing.T) {
	t.Parallel()

	catBin, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("cat not in PATH")
	}

	tests := []DiffTest{
		{
			Name:  "stdin_passthrough",
			Args:  []string{},
			Stdin: []byte("hello from stdin\n"),
		},
	}
	RunDiffTests(t, catBin, catBin, tests)
}

func TestRunDiffTestsWithWorkDir(t *testing.T) {
	t.Parallel()

	// Verify WorkDir is used correctly.
	pwdBin, err := exec.LookPath("pwd")
	if err != nil {
		t.Skip("pwd not in PATH")
	}

	dir := t.TempDir()
	tests := []DiffTest{
		{
			Name:    "workdir_set",
			Args:    []string{},
			WorkDir: dir,
		},
	}
	RunDiffTests(t, pwdBin, pwdBin, tests)
}

func TestRunDiffTestsWithNormalizer(t *testing.T) {
	t.Parallel()

	echoBin, err := exec.LookPath("echo")
	if err != nil {
		t.Skip("echo not in PATH")
	}

	// Use a normalizer that replaces all output with a constant.
	constNorm := func(data []byte) []byte {
		return []byte("normalized\n")
	}

	tests := []DiffTest{
		{
			Name:      "with_normalizer",
			Args:      []string{"anything"},
			Normalize: []NormalizeFunc{constNorm},
		},
	}
	RunDiffTests(t, echoBin, echoBin, tests)
}

func TestRunDiffTestsExpectedFiles(t *testing.T) {
	t.Parallel()

	// Create a simple script-like binary that writes a file.
	// We use /bin/sh to write a file, then verify ExpectedFiles checks it.
	shBin, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not in PATH")
	}

	dir := t.TempDir()
	tests := []DiffTest{
		{
			Name:    "file_output",
			Args:    []string{"-c", "echo hello > output.txt"},
			WorkDir: dir,
			ExpectedFiles: map[string][]byte{
				"output.txt": []byte("hello\n"),
			},
		},
	}
	RunDiffTests(t, shBin, shBin, tests)
}

func TestRunDiffTestsEmptyStdin(t *testing.T) {
	t.Parallel()

	// R1.2: empty non-nil slice produces no bytes.
	catBin, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("cat not in PATH")
	}

	tests := []DiffTest{
		{
			Name:  "empty_stdin",
			Args:  []string{},
			Stdin: []byte{},
		},
	}
	RunDiffTests(t, catBin, catBin, tests)
}

func TestBuildEnvDefaultLC(t *testing.T) {
	t.Parallel()

	// AC8: LC_ALL=C is set by default.
	env := buildEnv(nil)
	if !slices.Contains(env, "LC_ALL=C") {
		t.Error("buildEnv(nil) did not set LC_ALL=C")
	}
}

func TestBuildEnvOverrideLC(t *testing.T) {
	t.Parallel()

	// AC8: DiffTest.Env can override LC_ALL.
	env := buildEnv([]string{"LC_ALL=en_US.UTF-8"})
	if slices.Contains(env, "LC_ALL=C") {
		t.Error("buildEnv with LC_ALL override still has LC_ALL=C")
	}
	if !slices.Contains(env, "LC_ALL=en_US.UTF-8") {
		t.Error("buildEnv did not apply LC_ALL override")
	}
}

func TestBuildEnvMerge(t *testing.T) {
	t.Parallel()

	env := buildEnv([]string{"MY_VAR=hello"})
	if !slices.Contains(env, "MY_VAR=hello") {
		t.Error("buildEnv did not merge custom env var")
	}
}

func TestBuildBinaryNameExtraction(t *testing.T) {
	t.Parallel()

	// AC5: BuildBinary returns a usable binary path and cleans up.
	// Use an in-module cmd/ package that exists.
	binPath := BuildBinary(t, "github.com/petar-djukic/go-unix-utils/cmd/cat")
	if binPath == "" {
		t.Fatal("BuildBinary returned empty path")
	}
	// Verify the binary exists and is executable.
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("stat binary: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("BuildBinary output is not executable")
	}
	// Verify the binary name is derived from the package path.
	if filepath.Base(binPath) != "cat" {
		t.Errorf("binary name = %q, want %q", filepath.Base(binPath), "cat")
	}
}
