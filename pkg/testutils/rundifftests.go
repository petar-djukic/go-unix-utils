// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd001-testutils R2.1-R2.4: RunDiffTests binary execution and comparison.

package testutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// defaultTimeout is the per-binary invocation timeout.
// R2.3: configurable timeout with 10-second default.
const defaultTimeout = 10 * time.Second

// maxStdinReport is the maximum number of stdin bytes included in failure messages.
// R3.5: truncate stdin to 256 bytes in reports.
const maxStdinReport = 256

// lcAllDefault is the default locale environment variable set for both binaries.
// R2.6: LC_ALL=C unless explicitly overridden.
const lcAllDefault = "LC_ALL=C"

// RunDiffTests executes each DiffTest as a named subtest, running both goBinary
// and refBinary with identical Args, Stdin, Env, and WorkDir, then comparing
// stdout, stderr, and exit code.
//
// R2.1: iterate tests, call t.Run for each.
// R2.2: capture stdout, stderr, exit code independently.
// R2.4: no output on passing tests.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()

			// R2.5: determine working directory.
			workDir := tc.WorkDir
			if workDir == "" {
				workDir = t.TempDir()
			}

			// R2.6, R1.3: build environment with LC_ALL=C default.
			env := buildEnv(tc.Env)

			// Execute both binaries.
			refOut, refErr, refExit, refRunErr := runBinary(t, refBinary, tc.Args, tc.Stdin, env, workDir)
			if refRunErr != nil {
				t.Fatalf("reference binary failed to run: %v", refRunErr)
			}

			goOut, goErr, goExit, goRunErr := runBinary(t, goBinary, tc.Args, tc.Stdin, env, workDir)
			if goRunErr != nil {
				t.Fatalf("Go binary failed to run: %v", goRunErr)
			}

			// R3.1, R4.1, R4.3: apply normalization to both outputs.
			refOut = applyNormalizers(refOut, tc.Normalize)
			refErr = applyNormalizers(refErr, tc.Normalize)
			goOut = applyNormalizers(goOut, tc.Normalize)
			goErr = applyNormalizers(goErr, tc.Normalize)

			// R3.2: compare stdout.
			if !bytes.Equal(refOut, goOut) {
				t.Errorf("stdout mismatch\n"+
					"args:     %v\n"+
					"stdin:    %s\n"+
					"expected: %q\n"+
					"actual:   %q",
					tc.Args, truncateStdin(tc.Stdin), refOut, goOut)
			}

			// R3.3: compare stderr.
			if !bytes.Equal(refErr, goErr) {
				t.Errorf("stderr mismatch\n"+
					"args:     %v\n"+
					"stdin:    %s\n"+
					"expected: %q\n"+
					"actual:   %q",
					tc.Args, truncateStdin(tc.Stdin), refErr, goErr)
			}

			// R3.4, R2.4 (task R4): compare exit codes.
			if refExit != goExit {
				t.Errorf("exit code mismatch\n"+
					"args:     %v\n"+
					"expected: %d (reference)\n"+
					"actual:   %d (go)",
					tc.Args, refExit, goExit)
			}

			// R2.4 (task R4): verify both match expected exit code when set.
			if tc.ExitCode != 0 || refExit != 0 || goExit != 0 {
				if refExit != tc.ExitCode {
					t.Errorf("reference exit code %d does not match expected %d", refExit, tc.ExitCode)
				}
				if goExit != tc.ExitCode {
					t.Errorf("Go exit code %d does not match expected %d", goExit, tc.ExitCode)
				}
			}

			// R5.1, R5.2: compare expected files if specified.
			if tc.ExpectedFiles != nil {
				for path, expected := range tc.ExpectedFiles {
					fullPath := path
					if !strings.HasPrefix(path, "/") {
						fullPath = workDir + "/" + path
					}
					actual, err := os.ReadFile(fullPath)
					if err != nil {
						t.Errorf("expected file %s: %v", path, err)
						continue
					}
					if !bytes.Equal(expected, actual) {
						t.Errorf("file content mismatch for %s\n"+
							"expected: %q\n"+
							"actual:   %q",
							path, expected, actual)
					}
				}
			}
		})
	}
}

// runBinary executes a binary with the given arguments, stdin, environment, and
// working directory. Returns stdout, stderr, exit code, and any non-exit error.
//
// R2.2: capture stdout and stderr via bytes.Buffer.
// R2.3: impose defaultTimeout on each invocation.
// D4: extract exit code from exec.ExitError.
func runBinary(t *testing.T, binary string, args []string, stdin []byte, env []string, workDir string) (stdout, stderr []byte, exitCode int, err error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = env

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	runErr := cmd.Run()
	stdout = outBuf.Bytes()
	stderr = errBuf.Bytes()

	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return stdout, stderr, exitErr.ExitCode(), nil
		}
		// D4: non-ExitError means the command could not be started or timed out.
		return stdout, stderr, -1, runErr
	}

	return stdout, stderr, 0, nil
}

// buildEnv constructs the environment for binary execution.
// R2.6: sets LC_ALL=C by default. DiffTest.Env entries override or extend.
// R1.3: nil Env means inherit with defaults; non-nil merges into inherited env.
func buildEnv(testEnv []string) []string {
	base := os.Environ()

	// Check if test env explicitly sets LC_ALL.
	hasLCAll := false
	for _, e := range testEnv {
		if strings.HasPrefix(e, "LC_ALL=") {
			hasLCAll = true
			break
		}
	}

	// Apply LC_ALL=C default if not overridden.
	if !hasLCAll {
		base = setEnvVar(base, lcAllDefault)
	}

	// Merge test env entries.
	for _, e := range testEnv {
		base = setEnvVar(base, e)
	}

	return base
}

// setEnvVar sets or replaces an environment variable in the slice.
func setEnvVar(env []string, entry string) []string {
	key := entry[:strings.Index(entry, "=")+1]
	for i, e := range env {
		if strings.HasPrefix(e, key) {
			env[i] = entry
			return env
		}
	}
	return append(env, entry)
}

// applyNormalizers runs each NormalizeFunc in order on the input bytes.
// R3.1, R4.3: apply slice of normalizers in order; nil/empty = no-op.
func applyNormalizers(data []byte, fns []NormalizeFunc) []byte {
	for _, fn := range fns {
		data = fn(data)
	}
	return data
}

// truncateStdin returns stdin for display, truncated to maxStdinReport bytes.
// R3.5: truncate stdin to 256 bytes in failure messages.
func truncateStdin(stdin []byte) string {
	if stdin == nil {
		return "<nil>"
	}
	if len(stdin) > maxStdinReport {
		return fmt.Sprintf("%q... (%d bytes total)", stdin[:maxStdinReport], len(stdin))
	}
	return fmt.Sprintf("%q", stdin)
}
