// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It executes a Go binary and a GNU reference binary with identical inputs and
// compares stdout, stderr, and exit code.
// Implements srd001-testutils.
package testutils

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// NormalizeFunc transforms raw output bytes before comparison.
// R1.4: named type alias for normalization functions.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case.
//
// R1.1: Name identifies the subtest (used with t.Run). Args holds command-line
// arguments passed to both binaries.
//
// R1.2: Stdin nil means both binaries receive EOF immediately. An empty non-nil
// slice ([]byte{}) also produces no bytes but represents an open stdin that is
// immediately closed.
//
// R1.3: Env nil means both binaries inherit the test process environment with
// LC_ALL=C applied as a default override (R2.6). When non-nil, KEY=VALUE pairs
// are merged into the inherited environment: matching keys are overridden, new
// keys are added.
//
// R4.1, R4.3: Normalize holds a slice of NormalizeFunc applied in order to
// stdout and stderr of both binaries before comparison. Nil or empty means no
// normalization.
//
// R5.1: ExpectedFiles maps relative paths to expected byte content after
// execution, for file-output utilities.
type DiffTest struct {
	Name          string
	Args          []string
	Stdin         []byte
	Env           []string
	WorkDir       string
	ExitCode      int
	Normalize     []NormalizeFunc
	ExpectedFiles map[string][]byte
}

type execResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

const defaultTimeout = 10 * time.Second

var timestampRe = regexp.MustCompile(
	`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}` +
		`|[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}` +
		`|\d{2}:\d{2}:\d{2}`,
)

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder string.
// R4.2: built-in normalizer for cmd/ts tests.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	return timestampRe.ReplaceAll(b, []byte("<TIMESTAMP>"))
}

// RunDiffTests runs each DiffTest as a named subtest, executing goBinary and
// refBinary with identical inputs and comparing their outputs.
// R2.1, R2.2, R2.3, R2.4, R2.5, R2.6, R3.1, R3.2, R3.3, R3.4, R3.5, R3.6.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			workDir := tc.WorkDir
			if workDir == "" {
				workDir = t.TempDir()
			}
			env := buildEnv(tc.Env)
			ref := executeBinary(t, refBinary, tc, env, workDir)
			got := executeBinary(t, goBinary, tc, env, workDir)
			normalizeResults(tc.Normalize, ref, got)
			checkDivergence(t, tc, ref, got)
			checkExpectedFiles(t, tc.ExpectedFiles, workDir)
		})
	}
}

func executeBinary(t *testing.T, binary string, tc DiffTest, env []string, workDir string) *execResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, tc.Args...)
	cmd.Env = env
	cmd.Dir = workDir
	if tc.Stdin != nil {
		cmd.Stdin = bytes.NewReader(tc.Stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("timeout: %s exceeded %v", binary, defaultTimeout)
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return &execResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: ee.ExitCode()}
		}
		t.Fatalf("failed to run %s: %v", binary, err)
	}
	return &execResult{stdout: stdout.Bytes(), stderr: stderr.Bytes()}
}

func buildEnv(testEnv []string) []string {
	env := os.Environ()
	overrides := map[string]string{"LC_ALL": "C"}
	for _, e := range testEnv {
		if k, v, ok := strings.Cut(e, "="); ok {
			overrides[k] = v
		}
	}
	applied := make(map[string]bool)
	for i, e := range env {
		if k, _, ok := strings.Cut(e, "="); ok {
			if v, has := overrides[k]; has {
				env[i] = k + "=" + v
				applied[k] = true
			}
		}
	}
	for k, v := range overrides {
		if !applied[k] {
			env = append(env, k+"="+v)
		}
	}
	return env
}

func normalizeResults(norms []NormalizeFunc, results ...*execResult) {
	for _, fn := range norms {
		for _, r := range results {
			r.stdout = fn(r.stdout)
			r.stderr = fn(r.stderr)
		}
	}
}

func checkDivergence(t *testing.T, tc DiffTest, ref, got *execResult) {
	t.Helper()
	if bytes.Equal(ref.stdout, got.stdout) &&
		bytes.Equal(ref.stderr, got.stderr) &&
		ref.exitCode == got.exitCode {
		return
	}
	stdin := tc.Stdin
	if len(stdin) > 256 {
		stdin = stdin[:256]
	}
	t.Fatalf("divergence detected\nargs:       %v\nstdin:      %s\nref stdout: %s\ngo  stdout: %s\nref stderr: %s\ngo  stderr: %s\nref exit:   %d\ngo  exit:   %d",
		tc.Args, stdin, ref.stdout, got.stdout, ref.stderr, got.stderr, ref.exitCode, got.exitCode)
}

func checkExpectedFiles(t *testing.T, expected map[string][]byte, workDir string) {
	t.Helper()
	for relPath, want := range expected {
		got, err := os.ReadFile(filepath.Join(workDir, relPath))
		if err != nil {
			t.Fatalf("expected file %s: %v", relPath, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("file %s divergence:\nexpected: %s\nactual:   %s", relPath, want, got)
		}
	}
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order.
// R4.4: convenience for combining multiple normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}
