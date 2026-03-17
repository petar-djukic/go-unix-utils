// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// compare.go implements output comparison and diff reporting for the
// differential testing harness.
// Implements prd001-testutils R3.1-R3.5, R5.1-R5.2.

package testutils

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// maxStdinDisplay is the maximum bytes of stdin shown in failure messages.
// R3.5: stdin truncated to 256 bytes in divergence reports.
const maxStdinDisplay = 256

// compareResults checks stdout, stderr, and exit code for divergence.
// R3.2-R3.5: reports differences with full context.
func compareResults(
	t *testing.T, tc DiffTest,
	refStdout, goStdout, refStderr, goStderr []byte,
	refExit, goExit int,
) {
	t.Helper()
	if bytes.Equal(refStdout, goStdout) &&
		bytes.Equal(refStderr, goStderr) &&
		refExit == goExit {
		return
	}
	stdinDisplay := truncateBytes(tc.Stdin, maxStdinDisplay)
	reportDivergence(t, tc, stdinDisplay,
		refStdout, goStdout, refStderr, goStderr, refExit, goExit)
}

// reportDivergence formats and reports a test divergence.
func reportDivergence(
	t *testing.T, tc DiffTest, stdinDisplay []byte,
	refStdout, goStdout, refStderr, goStderr []byte,
	refExit, goExit int,
) {
	t.Helper()
	t.Errorf("divergence in %s\n"+
		"args: %v\nstdin: %q\n"+
		"ref stdout: %q\n go stdout: %q\n"+
		"ref stderr: %q\n go stderr: %q\n"+
		"ref exit: %d\n go exit: %d",
		tc.Name, tc.Args, stdinDisplay,
		refStdout, goStdout,
		refStderr, goStderr,
		refExit, goExit,
	)
}

// truncateBytes returns b truncated to maxLen bytes.
func truncateBytes(b []byte, maxLen int) []byte {
	if len(b) <= maxLen {
		return b
	}
	return b[:maxLen]
}

// checkExpectedFiles verifies file content after binary execution.
// R5.1-R5.2: compares expected file content byte-for-byte.
func checkExpectedFiles(t *testing.T, tc DiffTest, workDir string) {
	t.Helper()
	if tc.ExpectedFiles == nil {
		return
	}
	for relPath, expected := range tc.ExpectedFiles {
		checkSingleFile(t, workDir, relPath, expected)
	}
}

// checkSingleFile compares a single expected file against disk content.
func checkSingleFile(t *testing.T, workDir, relPath string, expected []byte) {
	t.Helper()
	fullPath := filepath.Join(workDir, relPath)
	actual, err := os.ReadFile(fullPath)
	if err != nil {
		t.Errorf("ExpectedFiles[%q]: %v", relPath, err)
		return
	}
	if !bytes.Equal(expected, actual) {
		t.Errorf("ExpectedFiles[%q] divergence:\n"+
			"expected: %q\n  actual: %q",
			relPath, expected, actual,
		)
	}
}
