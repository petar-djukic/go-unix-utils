// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeMockBinary creates a shell script in dir that acts as a mock binary.
func writeMockBinary(t *testing.T, dir, name, script string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	content := "#!/bin/sh\n" + script
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("writeMockBinary: %v", err)
	}
	return path
}

func TestComposeNormalizers(t *testing.T) {
	t.Parallel()

	upper := func(data []byte) []byte {
		return bytes.ToUpper(data)
	}
	addSuffix := func(data []byte) []byte {
		return append(data, []byte("_DONE")...)
	}

	composed := ComposeNormalizers(upper, addSuffix)
	result := composed([]byte("hello"))

	expected := []byte("HELLO_DONE")
	if !bytes.Equal(result, expected) {
		t.Fatalf("ComposeNormalizers: got %q, want %q", result, expected)
	}
}

func TestComposeNormalizersEmpty(t *testing.T) {
	t.Parallel()

	composed := ComposeNormalizers()
	input := []byte("unchanged")
	result := composed(input)

	if !bytes.Equal(result, input) {
		t.Fatalf("ComposeNormalizers(empty): got %q, want %q", result, input)
	}
}

func TestTimestampNormalizerISO(t *testing.T) {
	t.Parallel()

	input := []byte("event at 2026-03-07T12:34:56 happened")
	result := TimestampNormalizer(input)

	if bytes.Contains(result, []byte("2026-03-07")) {
		t.Fatalf("TimestampNormalizer did not replace ISO timestamp: %s", result)
	}
	if !bytes.Contains(result, []byte(timestampPlaceholder)) {
		t.Fatalf("TimestampNormalizer missing placeholder: %s", result)
	}
}

func TestTimestampNormalizerSyslog(t *testing.T) {
	t.Parallel()

	input := []byte("Feb 19 12:34:56 syslog message")
	result := TimestampNormalizer(input)

	if bytes.Contains(result, []byte("Feb 19")) {
		t.Fatalf("TimestampNormalizer did not replace syslog timestamp: %s", result)
	}
	if !bytes.Contains(result, []byte(timestampPlaceholder)) {
		t.Fatalf("TimestampNormalizer missing placeholder: %s", result)
	}
}

func TestTimestampNormalizerEpoch(t *testing.T) {
	t.Parallel()

	input := []byte("timestamp: 1709812345.123456 end")
	result := TimestampNormalizer(input)

	if bytes.Contains(result, []byte("1709812345")) {
		t.Fatalf("TimestampNormalizer did not replace epoch timestamp: %s", result)
	}
}

func TestTimestampNormalizerNoMatch(t *testing.T) {
	t.Parallel()

	input := []byte("no timestamps here")
	result := TimestampNormalizer(input)

	if !bytes.Equal(result, input) {
		t.Fatalf("TimestampNormalizer modified input without timestamps: got %q, want %q", result, input)
	}
}

func TestBuildEnvDefaultLC(t *testing.T) {
	t.Parallel()

	env := buildEnv(nil)

	found := false
	for _, entry := range env {
		if entry == "LC_ALL=C" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("buildEnv: LC_ALL=C not set by default")
	}
}

func TestBuildEnvOverrideLC(t *testing.T) {
	t.Parallel()

	env := buildEnv([]string{"LC_ALL=en_US.UTF-8"})

	for _, entry := range env {
		if entry == "LC_ALL=C" {
			t.Fatal("buildEnv: LC_ALL=C should be overridden")
		}
		if entry == "LC_ALL=en_US.UTF-8" {
			return
		}
	}
	t.Fatal("buildEnv: LC_ALL=en_US.UTF-8 not found")
}

func TestBuildEnvMerge(t *testing.T) {
	t.Parallel()

	env := buildEnv([]string{"MY_CUSTOM_VAR=hello"})

	found := false
	for _, entry := range env {
		if entry == "MY_CUSTOM_VAR=hello" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("buildEnv: custom env var not merged")
	}
}

func TestRunDiffTestsMatchingOutput(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	dir := t.TempDir()
	bin := writeMockBinary(t, dir, "mock", `echo "hello"`)

	tests := []DiffTest{
		{Name: "matching", Args: nil},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTestsZeroValue(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	dir := t.TempDir()
	// A binary that does nothing and exits 0.
	bin := writeMockBinary(t, dir, "noop", `exit 0`)

	tests := []DiffTest{
		{Name: "zero"},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTestsWithStdin(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	dir := t.TempDir()
	bin := writeMockBinary(t, dir, "cat", `cat`)

	tests := []DiffTest{
		{Name: "stdin", Stdin: []byte("input data\n")},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTestsWithNormalization(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	dir := t.TempDir()
	// Two binaries that produce different timestamps but same prefix.
	bin1 := writeMockBinary(t, dir, "ts1", `echo "event at 2026-03-07T12:00:00"`)
	bin2 := writeMockBinary(t, dir, "ts2", `echo "event at 2026-03-07T23:59:59"`)

	tests := []DiffTest{
		{
			Name:      "timestamp_normalized",
			Normalize: []NormalizeFunc{TimestampNormalizer},
		},
	}

	RunDiffTests(t, bin1, bin2, tests)
}

func TestRunDiffTestsExpectedFiles(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	dir := t.TempDir()
	// Both binaries write the same file content.
	bin := writeMockBinary(t, dir, "writer", `printf "file content" > "$PWD/output.txt"`)

	workDir := t.TempDir()
	tests := []DiffTest{
		{
			Name:          "file_output",
			WorkDir:       workDir,
			ExpectedFiles: map[string][]byte{"output.txt": []byte("file content")},
		},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTestsEnvLC(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	dir := t.TempDir()
	// Binary that prints the LC_ALL value.
	bin := writeMockBinary(t, dir, "printlc", `printenv LC_ALL`)

	// Both binaries get the same env, so outputs should match.
	tests := []DiffTest{
		{Name: "default_lc", Args: nil},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestRunDiffTestsEnvOverride(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	dir := t.TempDir()
	bin := writeMockBinary(t, dir, "printlc", `printenv LC_ALL`)

	tests := []DiffTest{
		{Name: "override_lc", Env: []string{"LC_ALL=en_US.UTF-8"}},
	}

	RunDiffTests(t, bin, bin, tests)
}

func TestApplyNormalizers(t *testing.T) {
	t.Parallel()

	upper := func(data []byte) []byte {
		return bytes.ToUpper(data)
	}
	trim := func(data []byte) []byte {
		return bytes.TrimSpace(data)
	}

	result := applyNormalizers([]byte("  hello  "), []NormalizeFunc{trim, upper})
	expected := []byte("HELLO")
	if !bytes.Equal(result, expected) {
		t.Fatalf("applyNormalizers: got %q, want %q", result, expected)
	}
}

func TestApplyNormalizersNil(t *testing.T) {
	t.Parallel()

	input := []byte("unchanged")
	result := applyNormalizers(input, nil)

	if !bytes.Equal(result, input) {
		t.Fatalf("applyNormalizers(nil): got %q, want %q", result, input)
	}
}

func TestTruncateBytes(t *testing.T) {
	t.Parallel()

	short := []byte("short")
	result := truncateBytes(short, 256)
	if !bytes.Equal(result, short) {
		t.Fatalf("truncateBytes(short): got %q, want %q", result, short)
	}

	long := bytes.Repeat([]byte("x"), 300)
	result = truncateBytes(long, 256)
	if len(result) != 256+len("...(truncated)") {
		t.Fatalf("truncateBytes(long): got len %d, want %d", len(result), 256+len("...(truncated)"))
	}
	if !strings.HasSuffix(string(result), "...(truncated)") {
		t.Fatalf("truncateBytes(long): missing truncation suffix: %s", result)
	}
}

func TestSetEnvVarNew(t *testing.T) {
	t.Parallel()

	env := []string{"PATH=/usr/bin", "HOME=/root"}
	env = setEnvVar(env, "MY_VAR", "hello")

	found := false
	for _, entry := range env {
		if entry == "MY_VAR=hello" {
			found = true
		}
	}
	if !found {
		t.Fatal("setEnvVar: new var not appended")
	}
}

func TestSetEnvVarOverride(t *testing.T) {
	t.Parallel()

	env := []string{"PATH=/usr/bin", "LC_ALL=C"}
	env = setEnvVar(env, "LC_ALL", "en_US.UTF-8")

	for _, entry := range env {
		if entry == "LC_ALL=C" {
			t.Fatal("setEnvVar: old value not overridden")
		}
	}

	found := false
	for _, entry := range env {
		if entry == "LC_ALL=en_US.UTF-8" {
			found = true
		}
	}
	if !found {
		t.Fatal("setEnvVar: new value not set")
	}
}

func TestRunDiffTestsWorkDir(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}

	dir := t.TempDir()
	// Binary that prints its working directory.
	bin := writeMockBinary(t, dir, "pwd", `pwd`)

	workDir := t.TempDir()
	tests := []DiffTest{
		{Name: "custom_workdir", WorkDir: workDir},
	}

	// Both binaries should print the same workDir.
	RunDiffTests(t, bin, bin, tests)
}
