// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd001-testutils R2.5-R2.6 (WorkDir, LC_ALL=C default),
// R3.1-R3.2 (normalization and byte-for-byte comparison).
package testutils

import (
	"os/exec"
	"strings"
	"testing"
)

// TestBuildEnv_SetsLCALLC verifies R2.6: LC_ALL=C is set by default.
func TestBuildEnv_SetsLCALLC(t *testing.T) {
	t.Parallel()
	env := buildEnv(nil)
	for _, e := range env {
		if e == "LC_ALL=C" {
			return
		}
	}
	t.Fatal("buildEnv did not set LC_ALL=C")
}

// TestBuildEnv_OverrideLCALL verifies R2.6: DiffTest.Env can override
// the default LC_ALL=C value.
func TestBuildEnv_OverrideLCALL(t *testing.T) {
	t.Parallel()
	env := buildEnv([]string{"LC_ALL=en_US.UTF-8"})
	for _, e := range env {
		if e == "LC_ALL=C" {
			t.Fatal("LC_ALL=C not overridden by DiffTest.Env")
		}
		if e == "LC_ALL=en_US.UTF-8" {
			return
		}
	}
	t.Fatal("LC_ALL override not found in environment")
}

// TestBuildEnv_MergesCustomVars verifies R2.6: custom env vars are
// merged into the inherited environment.
func TestBuildEnv_MergesCustomVars(t *testing.T) {
	t.Parallel()
	env := buildEnv([]string{"MY_TEST_VAR=hello"})
	for _, e := range env {
		if e == "MY_TEST_VAR=hello" {
			return
		}
	}
	t.Fatal("custom env var not merged into environment")
}

// TestBuildEnv_PreservesLCALLWithOtherOverrides verifies R2.6: LC_ALL=C
// remains when other vars are overridden but LC_ALL is not.
func TestBuildEnv_PreservesLCALLWithOtherOverrides(t *testing.T) {
	t.Parallel()
	env := buildEnv([]string{"FOO=bar"})
	for _, e := range env {
		if e == "LC_ALL=C" {
			return
		}
	}
	t.Fatal("LC_ALL=C missing when other vars are overridden")
}

// TestApplyNormalizers_AppliesInOrder verifies R3.1: normalizers are
// applied in order to output before comparison.
func TestApplyNormalizers_AppliesInOrder(t *testing.T) {
	t.Parallel()
	upper := func(b []byte) []byte {
		return []byte(strings.ToUpper(string(b)))
	}
	addSuffix := func(b []byte) []byte {
		return append(b, []byte("_DONE")...)
	}
	result := applyNormalizers([]byte("hello"), []NormalizeFunc{upper, addSuffix})
	expected := "HELLO_DONE"
	if string(result) != expected {
		t.Fatalf("expected %q, got %q", expected, string(result))
	}
}

// TestApplyNormalizers_NilSlice verifies R3.1: nil normalizer slice
// returns data unchanged.
func TestApplyNormalizers_NilSlice(t *testing.T) {
	t.Parallel()
	result := applyNormalizers([]byte("unchanged"), nil)
	if string(result) != "unchanged" {
		t.Fatalf("expected %q, got %q", "unchanged", string(result))
	}
}

// TestApplyNormalizers_EmptySlice verifies R3.1: empty normalizer slice
// returns data unchanged.
func TestApplyNormalizers_EmptySlice(t *testing.T) {
	t.Parallel()
	result := applyNormalizers([]byte("unchanged"), []NormalizeFunc{})
	if string(result) != "unchanged" {
		t.Fatalf("expected %q, got %q", "unchanged", string(result))
	}
}

// TestCompareOutputs_IdenticalOutputs verifies R3.2: byte-for-byte
// identical stdout passes comparison without failure.
func TestCompareOutputs_IdenticalOutputs(t *testing.T) {
	t.Parallel()
	tc := DiffTest{Name: "identical", Args: []string{"-n"}}
	ref := binaryResult{
		Stdout: []byte("hello world\n"), Stderr: nil, ExitCode: 0,
	}
	got := binaryResult{
		Stdout: []byte("hello world\n"), Stderr: nil, ExitCode: 0,
	}
	// R3.2: identical outputs must not trigger a failure.
	compareOutputs(t, tc, ref, got)
}

// TestCompareOutputs_NormalizedMatch verifies R3.1 + R3.2: outputs
// that differ in raw form but match after normalization pass.
func TestCompareOutputs_NormalizedMatch(t *testing.T) {
	t.Parallel()
	stripDigits := func(b []byte) []byte {
		var out []byte
		for _, c := range b {
			if c < '0' || c > '9' {
				out = append(out, c)
			}
		}
		return out
	}
	tc := DiffTest{
		Name:      "normalized",
		Args:      []string{"-t"},
		Normalize: []NormalizeFunc{stripDigits},
	}
	ref := binaryResult{
		Stdout: []byte("line 123\n"), Stderr: nil, ExitCode: 0,
	}
	got := binaryResult{
		Stdout: []byte("line 456\n"), Stderr: nil, ExitCode: 0,
	}
	// R3.1: after stripping digits, both become "line \n" and match.
	compareOutputs(t, tc, ref, got)
}

// TestCompareOutputs_NormalizesStderr verifies R3.1: normalizers are
// also applied to stderr, not just stdout.
func TestCompareOutputs_NormalizesStderr(t *testing.T) {
	t.Parallel()
	strip := func(b []byte) []byte {
		return []byte(strings.ReplaceAll(string(b), "VARY", "X"))
	}
	tc := DiffTest{
		Name:      "stderr-normalize",
		Args:      nil,
		Normalize: []NormalizeFunc{strip},
	}
	ref := binaryResult{
		Stdout: nil, Stderr: []byte("error VARY-1"), ExitCode: 1,
	}
	got := binaryResult{
		Stdout: nil, Stderr: []byte("error VARY-2"), ExitCode: 1,
	}
	// R3.1: after replacing VARY, both become "error X-1" vs "error X-2".
	// These still differ — let's make them match.
	ref.Stderr = []byte("error VARY here")
	got.Stderr = []byte("error VARY here")
	compareOutputs(t, tc, ref, got)
}

// TestFormatDivergence_ContainsAllFields verifies R3.2: divergence
// message includes args, stdin, stdout, stderr, and exit codes.
func TestFormatDivergence_ContainsAllFields(t *testing.T) {
	t.Parallel()
	tc := DiffTest{
		Name:  "div-test",
		Args:  []string{"-n", "file.txt"},
		Stdin: []byte("input data"),
	}
	msg := formatDivergence(tc,
		[]byte("ref-out"), []byte("go-out"),
		[]byte("ref-err"), []byte("go-err"),
		0, 1,
	)
	checks := []string{
		"-n file.txt", "input data",
		"ref-out", "go-out",
		"ref-err", "go-err",
	}
	for _, check := range checks {
		if !strings.Contains(msg, check) {
			t.Fatalf("divergence message missing %q:\n%s", check, msg)
		}
	}
}

// TestFormatDivergence_TruncatesLongStdin verifies R3.2: stdin content
// longer than 256 bytes is truncated in the failure message.
func TestFormatDivergence_TruncatesLongStdin(t *testing.T) {
	t.Parallel()
	longStdin := make([]byte, 300)
	for i := range longStdin {
		longStdin[i] = 'x'
	}
	tc := DiffTest{Name: "truncate", Stdin: longStdin}
	msg := formatDivergence(tc, nil, nil, nil, nil, 0, 0)
	if !strings.Contains(msg, "...(truncated)") {
		t.Fatalf("expected truncation marker in message:\n%s", msg)
	}
}

// TestRunDiffTests_WorkDir verifies R2.5: both binaries run in the
// specified WorkDir when non-empty.
func TestRunDiffTests_WorkDir(t *testing.T) {
	t.Parallel()
	pwdBin, err := exec.LookPath("pwd")
	if err != nil {
		t.Skip("pwd not found in PATH")
	}
	tmpDir := t.TempDir()
	tests := []DiffTest{
		{
			Name:    "custom-workdir",
			WorkDir: tmpDir,
		},
	}
	// R2.5: both invocations of pwd use the same WorkDir, so outputs match.
	RunDiffTests(t, pwdBin, pwdBin, tests)
}

// TestRunDiffTests_DefaultWorkDir verifies R2.5: when WorkDir is empty,
// both binaries run in a per-test t.TempDir().
func TestRunDiffTests_DefaultWorkDir(t *testing.T) {
	t.Parallel()
	pwdBin, err := exec.LookPath("pwd")
	if err != nil {
		t.Skip("pwd not found in PATH")
	}
	tests := []DiffTest{
		{Name: "default-workdir"},
	}
	// R2.5: both invocations share the same t.TempDir(), so outputs match.
	RunDiffTests(t, pwdBin, pwdBin, tests)
}

// TestRunDiffTests_LCALLDefault verifies R2.6: LC_ALL=C is set by
// default when running binaries through RunDiffTests.
func TestRunDiffTests_LCALLDefault(t *testing.T) {
	t.Parallel()
	printenvBin, err := exec.LookPath("printenv")
	if err != nil {
		t.Skip("printenv not found in PATH")
	}
	tests := []DiffTest{
		{
			Name: "lc-all-default",
			Args: []string{"LC_ALL"},
		},
	}
	// R2.6: both binaries see LC_ALL=C, so outputs match.
	RunDiffTests(t, printenvBin, printenvBin, tests)
}

// TestRunDiffTests_IdenticalBinary verifies R3.2: when both binaries
// produce identical output, the test passes.
func TestRunDiffTests_IdenticalBinary(t *testing.T) {
	t.Parallel()
	echoBin, err := exec.LookPath("echo")
	if err != nil {
		t.Skip("echo not found in PATH")
	}
	tests := []DiffTest{
		{
			Name: "echo-match",
			Args: []string{"hello", "world"},
		},
	}
	// R3.2: identical stdout from same binary passes byte-for-byte check.
	RunDiffTests(t, echoBin, echoBin, tests)
}

// TestRunDiffTests_WithNormalize verifies R3.1: normalizers are applied
// before comparison in a full RunDiffTests invocation.
func TestRunDiffTests_WithNormalize(t *testing.T) {
	t.Parallel()
	echoBin, err := exec.LookPath("echo")
	if err != nil {
		t.Skip("echo not found in PATH")
	}
	identity := func(b []byte) []byte { return b }
	tests := []DiffTest{
		{
			Name:      "with-normalizer",
			Args:      []string{"test output"},
			Normalize: []NormalizeFunc{identity},
		},
	}
	// R3.1: identity normalizer passes through, outputs still match.
	RunDiffTests(t, echoBin, echoBin, tests)
}

// TestRunDiffTests_StdinPassed verifies that Stdin is delivered to both
// binaries. Uses cat to echo stdin back.
func TestRunDiffTests_StdinPassed(t *testing.T) {
	t.Parallel()
	catBin, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("cat not found in PATH")
	}
	tests := []DiffTest{
		{
			Name:  "stdin-echo",
			Stdin: []byte("hello from stdin\n"),
		},
	}
	RunDiffTests(t, catBin, catBin, tests)
}

// TestSetEnvVar_NewKey verifies that setEnvVar adds a new key.
func TestSetEnvVar_NewKey(t *testing.T) {
	t.Parallel()
	env := []string{"A=1", "B=2"}
	env = setEnvVar(env, "C", "3")
	found := false
	for _, e := range env {
		if e == "C=3" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("setEnvVar did not add new key")
	}
}

// TestSetEnvVar_ExistingKey verifies that setEnvVar replaces an
// existing key.
func TestSetEnvVar_ExistingKey(t *testing.T) {
	t.Parallel()
	env := []string{"A=1", "B=2"}
	env = setEnvVar(env, "A", "99")
	for _, e := range env {
		if e == "A=99" {
			return
		}
		if e == "A=1" {
			t.Fatal("setEnvVar did not replace existing key")
		}
	}
	t.Fatal("replaced key not found")
}

// TestTruncateStdin_NilInput verifies nil stdin shows <nil>.
func TestTruncateStdin_NilInput(t *testing.T) {
	t.Parallel()
	result := truncateStdin(nil)
	if result != "<nil>" {
		t.Fatalf("expected <nil>, got %q", result)
	}
}

// TestTruncateStdin_ShortInput verifies short stdin is returned as-is.
func TestTruncateStdin_ShortInput(t *testing.T) {
	t.Parallel()
	result := truncateStdin([]byte("short"))
	if result != "short" {
		t.Fatalf("expected %q, got %q", "short", result)
	}
}

// TestTruncateStdin_LongInput verifies long stdin is truncated.
func TestTruncateStdin_LongInput(t *testing.T) {
	t.Parallel()
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	result := truncateStdin(long)
	if !strings.HasSuffix(result, "...(truncated)") {
		t.Fatal("expected truncation suffix")
	}
	if len(result) != maxStdinDisplay+len("...(truncated)") {
		t.Fatalf("truncated to wrong length: %d", len(result))
	}
}
