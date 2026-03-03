// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Unit tests for the differential testing harness (prd001-testutils R1-R4).
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

// testBinPath is the default testbin binary (all ldflags defaults empty).
// testBinAltPath has ldflags-injected defaults for divergence testing:
//
//	defaultStdout="alt-stdout", defaultStderr="alt-stderr", defaultExitCode="1"
var (
	testBinPath    string
	testBinAltPath string
)

// expectFailEnv is the sentinel environment variable for subprocess divergence
// tests. When set to "1", the test function enters its divergence branch
// instead of re-executing as a subprocess.
const expectFailEnv = "TESTUTIL_EXPECT_FAIL"

// altLdflags injects non-empty defaults into testbin-alt so that it produces
// different output from testbin when no flags override the defaults.
const altLdflags = "-X main.defaultStdout=alt-stdout -X main.defaultStderr=alt-stderr -X main.defaultExitCode=1"

func TestMain(m *testing.M) {
	// When running as a subprocess for divergence tests, reuse pre-built
	// binaries passed via environment variables to avoid rebuilding.
	if path := os.Getenv("TESTBIN_PATH"); path != "" {
		testBinPath = path
		testBinAltPath = os.Getenv("TESTBIN_ALT_PATH")
		os.Exit(m.Run())
	}

	tmpDir, err := os.MkdirTemp("", "testutils-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir: %v\n", err)
		os.Exit(1)
	}

	testBinPath = filepath.Join(tmpDir, "testbin")
	if err := buildHelper(testBinPath, ""); err != nil {
		fmt.Fprintf(os.Stderr, "building testbin: %v\n", err)
		os.RemoveAll(tmpDir) // best-effort cleanup
		os.Exit(1)
	}

	testBinAltPath = filepath.Join(tmpDir, "testbin-alt")
	if err := buildHelper(testBinAltPath, altLdflags); err != nil {
		fmt.Fprintf(os.Stderr, "building testbin-alt: %v\n", err)
		os.RemoveAll(tmpDir) // best-effort cleanup
		os.Exit(1)
	}

	os.Setenv("TESTBIN_PATH", testBinPath)
	os.Setenv("TESTBIN_ALT_PATH", testBinAltPath)

	code := m.Run()
	os.RemoveAll(tmpDir) // best-effort cleanup
	os.Exit(code)
}

// buildHelper compiles testdata/testbin with optional ldflags to outputPath.
func buildHelper(outputPath, ldflags string) error {
	args := []string{"build", "-o", outputPath}
	if ldflags != "" {
		args = append(args, "-ldflags", ldflags)
	}
	args = append(args, "./testdata/testbin")
	cmd := exec.Command("go", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

// ---------------------------------------------------------------------------
// R1: RunDiffTests passes when both binaries produce identical output
//     (prd001-testutils R2)
// ---------------------------------------------------------------------------

func TestRunDiffTests_MatchingOutput(t *testing.T) {
	tests := []DiffTest{
		{Name: "empty-output"},
		{Name: "stdout-only", Args: []string{"--stdout", "hello\n"}},
		{Name: "stderr-only", Args: []string{"--stderr", "warning\n"}},
		{Name: "combined", Args: []string{"--stdout", "out\n", "--stderr", "err\n"}},
		{Name: "nonzero-exit", Args: []string{"--exit-code", "2"}},
		{Name: "with-stdin", Args: []string{"--stdout", "data\n"}, Stdin: "input"},
	}
	RunDiffTests(t, testBinPath, testBinPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R2: RunDiffTests detects divergence in stdout, stderr, or exit code
//     (prd001-testutils R2, R3)
//
// These tests use the subprocess pattern: the test re-executes itself with
// TESTUTIL_EXPECT_FAIL=1, enters the divergent branch, and the outer test
// verifies the subprocess failed with a "divergence" message.
// ---------------------------------------------------------------------------

func TestRunDiffTests_DivergentStdout(t *testing.T) {
	if os.Getenv(expectFailEnv) == "1" {
		// Subprocess: only stdout differs (stderr and exit overridden to match).
		RunDiffTests(t, testBinPath, testBinAltPath, nil, []DiffTest{
			{Name: "div-stdout", Args: []string{"--stderr", "", "--exit-code", "0"}},
		})
		return
	}
	out := runExpectFail(t, "TestRunDiffTests_DivergentStdout")
	if !bytes.Contains(out, []byte("divergence")) {
		t.Fatalf("expected divergence message, got:\n%s", out)
	}
}

func TestRunDiffTests_DivergentStderr(t *testing.T) {
	if os.Getenv(expectFailEnv) == "1" {
		// Subprocess: only stderr differs (stdout and exit overridden to match).
		RunDiffTests(t, testBinPath, testBinAltPath, nil, []DiffTest{
			{Name: "div-stderr", Args: []string{"--stdout", "", "--exit-code", "0"}},
		})
		return
	}
	out := runExpectFail(t, "TestRunDiffTests_DivergentStderr")
	if !bytes.Contains(out, []byte("divergence")) {
		t.Fatalf("expected divergence message, got:\n%s", out)
	}
}

func TestRunDiffTests_DivergentExitCode(t *testing.T) {
	if os.Getenv(expectFailEnv) == "1" {
		// Subprocess: only exit code differs (stdout and stderr overridden to match).
		RunDiffTests(t, testBinPath, testBinAltPath, nil, []DiffTest{
			{Name: "div-exit", Args: []string{"--stdout", "", "--stderr", ""}},
		})
		return
	}
	out := runExpectFail(t, "TestRunDiffTests_DivergentExitCode")
	if !bytes.Contains(out, []byte("divergence")) {
		t.Fatalf("expected divergence message, got:\n%s", out)
	}
}

// runExpectFail re-executes the named test as a subprocess and expects it to
// fail. It returns the combined stdout+stderr from the subprocess.
func runExpectFail(t *testing.T, testName string) []byte {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+testName+"$", "-test.v")
	cmd.Env = append(os.Environ(), expectFailEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected subprocess test %s to fail", testName)
	}
	return out
}

// ---------------------------------------------------------------------------
// R3: buildEnv and setEnvVar (prd001-testutils R3, ARCHITECTURE DD6)
// ---------------------------------------------------------------------------

func TestBuildEnv(t *testing.T) {
	tests := []struct {
		name    string
		testEnv []string
		key     string
		want    string
	}{
		{
			name: "default_LC_ALL_C",
			key:  "LC_ALL",
			want: "C",
		},
		{
			name:    "override_LC_ALL",
			testEnv: []string{"LC_ALL=en_US.UTF-8"},
			key:     "LC_ALL",
			want:    "en_US.UTF-8",
		},
		{
			name:    "custom_variable_added",
			testEnv: []string{"MY_CUSTOM=hello"},
			key:     "MY_CUSTOM",
			want:    "hello",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := buildEnv(tc.testEnv)
			got := envLookup(env, tc.key)
			if got != tc.want {
				t.Errorf("expected %s=%s, got %s=%s", tc.key, tc.want, tc.key, got)
			}
		})
	}
}

func TestBuildEnv_LC_ALL_Preserved_With_Custom(t *testing.T) {
	env := buildEnv([]string{"OTHER=value"})
	if envLookup(env, "LC_ALL") != "C" {
		t.Error("LC_ALL=C must be preserved when adding unrelated variables")
	}
}

func TestSetEnvVar(t *testing.T) {
	tests := []struct {
		name    string
		initial []string
		key     string
		value   string
		want    []string
	}{
		{
			name:    "replace_existing",
			initial: []string{"FOO=old", "BAR=baz"},
			key:     "FOO",
			value:   "new",
			want:    []string{"FOO=new", "BAR=baz"},
		},
		{
			name:    "append_new",
			initial: []string{"FOO=bar"},
			key:     "NEW",
			value:   "val",
			want:    []string{"FOO=bar", "NEW=val"},
		},
		{
			name:    "replace_middle",
			initial: []string{"A=1", "B=2", "C=3"},
			key:     "B",
			value:   "updated",
			want:    []string{"A=1", "B=updated", "C=3"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := make([]string, len(tc.initial))
			copy(env, tc.initial)
			got := setEnvVar(env, tc.key, tc.value)
			if len(got) != len(tc.want) {
				t.Fatalf("expected %d entries, got %d: %v", len(tc.want), len(got), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("entry %d: expected %q, got %q", i, tc.want[i], got[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// R4: NormalizeFunc application (prd001-testutils R4)
// ---------------------------------------------------------------------------

func TestRunDiffTests_NormalizerReconcilesDivergence(t *testing.T) {
	// testBinPath outputs "" on stdout; testBinAltPath outputs "alt-stdout".
	// A normalizer that strips "alt-stdout" reconciles the difference.
	normalizer := func(b []byte) []byte {
		return bytes.ReplaceAll(b, []byte("alt-stdout"), nil)
	}
	RunDiffTests(t, testBinPath, testBinAltPath, []NormalizeFunc{normalizer}, []DiffTest{
		{Name: "reconciled", Args: []string{"--stderr", "", "--exit-code", "0"}},
	})
}

func TestRunDiffTests_MultipleNormalizersAppliedInOrder(t *testing.T) {
	// First normalizer strips "alt-", second strips "stdout".
	// Combined: "alt-stdout" -> "stdout" -> "".
	first := func(b []byte) []byte {
		return bytes.ReplaceAll(b, []byte("alt-"), nil)
	}
	second := func(b []byte) []byte {
		return bytes.ReplaceAll(b, []byte("stdout"), nil)
	}
	RunDiffTests(t, testBinPath, testBinAltPath, []NormalizeFunc{first, second}, []DiffTest{
		{Name: "multi-normalizer", Args: []string{"--stderr", "", "--exit-code", "0"}},
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// envLookup returns the value for key in the environment slice, or "" if absent.
func envLookup(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):]
		}
	}
	return ""
}
