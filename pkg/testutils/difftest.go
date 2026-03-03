// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for verifying Go
// utility implementations against GNU reference binaries.
//
// Implements prd001-testutils R1, R2, R3, R4.
package testutils

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// defaultTimeout is the per-binary execution timeout (prd001-testutils R2.3).
const defaultTimeout = 10 * time.Second

// DiffTest represents a single differential test case per ARCHITECTURE.yaml.
// Both the Go binary under test and the GNU reference binary are invoked with
// identical Args, Stdin, and Env for each test case.
type DiffTest struct {
	// Name is the subtest name used with t.Run.
	Name string
	// Args are command-line arguments passed to both binaries.
	Args []string
	// Stdin is standard input content sent to both binaries. Empty means no stdin.
	Stdin string
	// Env holds KEY=VALUE pairs merged into the inherited environment.
	// nil means defaults only (LC_ALL=C).
	Env []string
}

// NormalizeFunc transforms output bytes before comparison. Normalization
// functions strip or replace non-deterministic output fields (timestamps,
// absolute paths) so that differential comparison succeeds despite expected
// variation.
type NormalizeFunc = func([]byte) []byte

// RunDiffTests executes each DiffTest as a named subtest, running both the Go
// binary and the reference GNU binary with identical inputs. It compares stdout,
// stderr, and exit code, reporting any divergence via t.Errorf.
//
// normalizers are applied in order to stdout and stderr of both binaries before
// comparison. Pass nil for no normalization.
//
// LC_ALL=C is prepended to the environment for both binaries unless overridden
// by DiffTest.Env (design decision DD6).
func RunDiffTests(t *testing.T, goBinary, refBinary string, normalizers []NormalizeFunc, tests []DiffTest) {
	t.Helper()

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()

			env := buildEnv(tc.Env)

			refStdout, refStderr, refExit := runBinary(t, refBinary, tc.Args, tc.Stdin, env)
			goStdout, goStderr, goExit := runBinary(t, goBinary, tc.Args, tc.Stdin, env)

			for _, fn := range normalizers {
				refStdout = fn(refStdout)
				goStdout = fn(goStdout)
				refStderr = fn(refStderr)
				goStderr = fn(goStderr)
			}

			if !bytes.Equal(refStdout, goStdout) || !bytes.Equal(refStderr, goStderr) || refExit != goExit {
				t.Errorf("divergence for %q\n"+
					"  args:        %v\n"+
					"  stdin:       %q\n"+
					"  ref stdout:  %q\n"+
					"  go  stdout:  %q\n"+
					"  ref stderr:  %q\n"+
					"  go  stderr:  %q\n"+
					"  ref exit:    %d\n"+
					"  go  exit:    %d",
					tc.Name, tc.Args, tc.Stdin,
					refStdout, goStdout,
					refStderr, goStderr,
					refExit, goExit,
				)
			}
		})
	}
}

// buildEnv constructs the environment for binary execution. It starts with the
// current process environment, sets LC_ALL=C as a default, then merges any
// per-test overrides from DiffTest.Env.
func buildEnv(testEnv []string) []string {
	env := os.Environ()
	env = setEnvVar(env, "LC_ALL", "C")

	for _, kv := range testEnv {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env = setEnvVar(env, k, v)
		}
	}

	return env
}

// setEnvVar sets key=value in the environment slice, replacing an existing entry
// for the same key or appending if not present.
func setEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}

	return append(env, prefix+value)
}

// runBinary executes a binary with the given arguments, stdin, and environment,
// returning its stdout, stderr, and exit code.
func runBinary(t *testing.T, binary string, args []string, stdin string, env []string) (stdout, stderr []byte, exitCode int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = env

	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()

	stdout = outBuf.Bytes()
	stderr = errBuf.Bytes()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("timeout executing %s with args %v", binary, args)
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to execute %s: %v", binary, err)
		}
	}

	return stdout, stderr, exitCode
}
