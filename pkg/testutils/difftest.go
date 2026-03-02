// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It executes a Go binary and a GNU reference binary with identical inputs and
// compares stdout, stderr, and exit code. cmd/ packages import this package to
// define test cases as data without reimplementing the execution and comparison
// logic.
//
// Implements: prd001-testutils R1, R2, R3, R4, R5
package testutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// defaultTimeout is the maximum duration each binary invocation may run before
// the harness kills it and fails the test.
//
// Implements: prd001-testutils R2.3
const defaultTimeout = 10 * time.Second

// NormalizeFunc transforms raw output bytes before comparison. Normalizers
// strip or replace non-deterministic fields (timestamps, PIDs, binary name
// references) so that known acceptable differences do not cause test failures.
//
// Implements: prd001-testutils R1.4
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. The harness executes
// both the Go binary and the reference binary with identical Args, Stdin,
// and Env, then compares their outputs.
//
// Implements: prd001-testutils R1.1
type DiffTest struct {
	Name          string            // subtest name used with t.Run; required
	Args          []string          // command-line arguments passed to both binaries
	Stdin         []byte            // nil = no stdin (both binaries receive EOF immediately)
	Env           []string          // nil = use defaults only (LC_ALL=C); non-nil = KEY=VALUE pairs merged into inherited environment
	WorkDir       string            // empty = per-test t.TempDir(); non-empty = use this directory for both binaries
	ExitCode      int               // expected exit code for both binaries
	Normalize     []NormalizeFunc   // applied in order to stdout and stderr of both binaries before comparison; nil or empty = no normalization
	ExpectedFiles map[string][]byte // optional: path -> expected byte content after execution, for file-output utilities (sponge, cp)
}

// stdinMaxDisplay is the maximum number of stdin bytes shown in failure messages.
const stdinMaxDisplay = 256

// runResult holds captured output from a single binary invocation.
type runResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// RunDiffTests runs each DiffTest as a named subtest, executing both goBinary
// and refBinary with identical inputs. Divergences in stdout, stderr, or exit
// code are reported via t.Errorf.
//
// Implements: prd001-testutils R2.1, R3.6
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleDiffTest(t, goBinary, refBinary, tc)
		})
	}
}

// runSingleDiffTest executes both binaries for a single test case and compares
// the results.
func runSingleDiffTest(t *testing.T, goBinary, refBinary string, tc DiffTest) {
	t.Helper()

	workDir := tc.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}

	env := buildEnv(tc.Env)

	refResult, err := runBinary(refBinary, tc.Args, tc.Stdin, env, workDir)
	if err != nil {
		t.Fatalf("reference binary execution failed: %v", err)
	}

	goResult, err := runBinary(goBinary, tc.Args, tc.Stdin, env, workDir)
	if err != nil {
		t.Fatalf("go binary execution failed: %v", err)
	}

	refStdout := applyNormalizers(refResult.Stdout, tc.Normalize)
	refStderr := applyNormalizers(refResult.Stderr, tc.Normalize)
	goStdout := applyNormalizers(goResult.Stdout, tc.Normalize)
	goStderr := applyNormalizers(goResult.Stderr, tc.Normalize)

	failed := false

	// Compare stdout (R3.2).
	if !bytes.Equal(refStdout, goStdout) {
		failed = true
	}

	// Compare stderr (R3.3).
	if !bytes.Equal(refStderr, goStderr) {
		failed = true
	}

	// Compare exit codes (R3.4).
	if refResult.ExitCode != goResult.ExitCode {
		failed = true
	}

	if failed {
		t.Errorf("divergence detected\n"+
			"args:        %v\n"+
			"stdin:       %s\n"+
			"ref stdout:  %q\n"+
			"go  stdout:  %q\n"+
			"ref stderr:  %q\n"+
			"go  stderr:  %q\n"+
			"ref exit:    %d\n"+
			"go  exit:    %d",
			tc.Args,
			truncateStdin(tc.Stdin),
			refStdout,
			goStdout,
			refStderr,
			goStderr,
			refResult.ExitCode,
			goResult.ExitCode,
		)
	}

	// File-state comparison (R5.1, R5.2).
	if tc.ExpectedFiles != nil {
		checkExpectedFiles(t, tc, workDir)
	}
}

// buildEnv constructs the environment variable slice for binary invocations.
// It starts with the current process environment, applies LC_ALL=C as a
// default, then merges any DiffTest.Env entries on top.
//
// Implements: prd001-testutils R2.6, R1.3
func buildEnv(testEnv []string) []string {
	base := os.Environ()
	env := mergeEnv(base, []string{"LC_ALL=C"})
	if testEnv != nil {
		env = mergeEnv(env, testEnv)
	}
	return env
}

// mergeEnv merges override entries into base. For each override KEY=VALUE,
// if KEY exists in base it is replaced; otherwise the entry is appended.
func mergeEnv(base, overrides []string) []string {
	result := make([]string, len(base))
	copy(result, base)

	for _, override := range overrides {
		key := envKey(override)
		found := false
		for i, entry := range result {
			if envKey(entry) == key {
				result[i] = override
				found = true
				break
			}
		}
		if !found {
			result = append(result, override)
		}
	}
	return result
}

// envKey extracts the key portion of a KEY=VALUE environment string.
func envKey(entry string) string {
	if idx := strings.IndexByte(entry, '='); idx >= 0 {
		return entry[:idx]
	}
	return entry
}

// runBinary executes a binary with the given arguments, stdin, environment,
// and working directory. It captures stdout, stderr, and exit code.
//
// Implements: prd001-testutils R2.2, R2.3, R2.5
func runBinary(binPath string, args []string, stdin []byte, env []string, workDir string) (*runResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Env = env
	cmd.Dir = workDir

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("binary %s timed out after %v", binPath, defaultTimeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("executing %s: %w", binPath, err)
		}
	}

	return &runResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: exitCode,
	}, nil
}

// applyNormalizers runs each NormalizeFunc in order on the input bytes.
// Returns the input unchanged when fns is nil or empty.
//
// Implements: prd001-testutils R3.1, R4.1, R4.3
func applyNormalizers(data []byte, fns []NormalizeFunc) []byte {
	for _, fn := range fns {
		data = fn(data)
	}
	return data
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order. This is a convenience for cmd/ test files that combine
// multiple normalizers.
//
// Implements: prd001-testutils R4.4
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			data = fn(data)
		}
		return data
	}
}

// TimestampNormalizer replaces common strftime-formatted timestamps with a
// fixed placeholder so that differential tests pass despite wall-clock
// differences between binary invocations.
//
// Recognized patterns:
//   - "Mon DD HH:MM:SS" (e.g., "Feb 19 12:34:56") — syslog/ts default format
//   - "YYYY-MM-DD HH:MM:SS" (e.g., "2026-02-19 12:34:56") — ISO-like format
//
// Implements: prd001-testutils R4.2
func TimestampNormalizer(data []byte) []byte {
	s := string(data)

	// Replace "Mon DD HH:MM:SS" pattern (e.g., "Feb 19 12:34:56").
	s = replaceTimestampSyslog(s)

	// Replace "YYYY-MM-DD HH:MM:SS" pattern.
	s = replaceTimestampISO(s)

	return []byte(s)
}

// timestampPlaceholder is the fixed string that replaces normalized timestamps.
const timestampPlaceholder = "<TIMESTAMP>"

// months lists abbreviated month names for syslog timestamp matching.
var months = []string{
	"Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
}

// replaceTimestampSyslog replaces "Mon DD HH:MM:SS" patterns.
func replaceTimestampSyslog(s string) string {
	for _, mon := range months {
		for {
			idx := strings.Index(s, mon+" ")
			if idx < 0 {
				break
			}
			// Expect "Mon DD HH:MM:SS" = 3 + 1 + 2 + 1 + 8 = 15 chars.
			end := idx + 15
			if end > len(s) {
				break
			}
			candidate := s[idx:end]
			if isValidSyslogTimestamp(candidate) {
				s = s[:idx] + timestampPlaceholder + s[end:]
			} else {
				// Avoid infinite loop: advance past this occurrence.
				break
			}
		}
	}
	return s
}

// isValidSyslogTimestamp checks if s matches "Mon DD HH:MM:SS" format.
func isValidSyslogTimestamp(s string) bool {
	if len(s) != 15 {
		return false
	}
	// s[0:3] = month abbreviation (already matched by caller).
	if s[3] != ' ' {
		return false
	}
	// s[4:6] = day: digit or space + digit.
	if !isDigitOrSpace(s[4]) || !isDigit(s[5]) {
		return false
	}
	if s[6] != ' ' {
		return false
	}
	// s[7:15] = "HH:MM:SS"
	return isDigit(s[7]) && isDigit(s[8]) && s[9] == ':' &&
		isDigit(s[10]) && isDigit(s[11]) && s[12] == ':' &&
		isDigit(s[13]) && isDigit(s[14])
}

// replaceTimestampISO replaces "YYYY-MM-DD HH:MM:SS" patterns.
func replaceTimestampISO(s string) string {
	for {
		idx := findISOTimestamp(s)
		if idx < 0 {
			break
		}
		// "YYYY-MM-DD HH:MM:SS" = 19 chars.
		end := idx + 19
		s = s[:idx] + timestampPlaceholder + s[end:]
	}
	return s
}

// findISOTimestamp returns the index of the first "YYYY-MM-DD HH:MM:SS"
// pattern in s, or -1 if none found.
func findISOTimestamp(s string) int {
	// Minimum length is 19 chars.
	for i := 0; i+19 <= len(s); i++ {
		if isDigit(s[i]) && isDigit(s[i+1]) && isDigit(s[i+2]) && isDigit(s[i+3]) &&
			s[i+4] == '-' && isDigit(s[i+5]) && isDigit(s[i+6]) &&
			s[i+7] == '-' && isDigit(s[i+8]) && isDigit(s[i+9]) &&
			s[i+10] == ' ' &&
			isDigit(s[i+11]) && isDigit(s[i+12]) && s[i+13] == ':' &&
			isDigit(s[i+14]) && isDigit(s[i+15]) && s[i+16] == ':' &&
			isDigit(s[i+17]) && isDigit(s[i+18]) {
			return i
		}
	}
	return -1
}

// isDigit reports whether b is an ASCII digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// isDigitOrSpace reports whether b is an ASCII digit or a space.
func isDigitOrSpace(b byte) bool {
	return isDigit(b) || b == ' '
}

// truncateStdin returns a display-safe representation of stdin, truncated to
// stdinMaxDisplay bytes if longer.
func truncateStdin(stdin []byte) string {
	if stdin == nil {
		return "<nil>"
	}
	if len(stdin) <= stdinMaxDisplay {
		return fmt.Sprintf("%q", stdin)
	}
	return fmt.Sprintf("%q... (%d bytes total)", stdin[:stdinMaxDisplay], len(stdin))
}

// checkExpectedFiles verifies that files written by the Go binary match the
// expected content specified in DiffTest.ExpectedFiles.
//
// Implements: prd001-testutils R5.1, R5.2
func checkExpectedFiles(t *testing.T, tc DiffTest, workDir string) {
	t.Helper()

	for relPath, expected := range tc.ExpectedFiles {
		absPath := filepath.Join(workDir, relPath)
		actual, err := os.ReadFile(absPath)
		if err != nil {
			t.Errorf("expected file %s: %v", relPath, err)
			continue
		}
		if !bytes.Equal(expected, actual) {
			t.Errorf("file content divergence for %s\n"+
				"expected: %q\n"+
				"actual:   %q",
				relPath, expected, actual,
			)
		}
	}
}
