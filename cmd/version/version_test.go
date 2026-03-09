// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/version (prd059-version R1.1–R1.5).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testBinary compiles cmd/version to a temporary binary and returns its path.
func testBinary(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "version")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run(), "go build cmd/version")
	return binPath
}

// testBinaryWithVersion compiles cmd/version with a specific version injected
// via ldflags and returns the binary path.
func testBinaryWithVersion(t *testing.T, ver string) string {
	t.Helper()
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "version")
	ldflags := "-X main.version=" + ver
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", binPath, ".")
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run(), "go build cmd/version with ldflags")
	return binPath
}

// TestNoArgs_PrintsDev verifies that a dev build (no ldflags) prints "dev"
// followed by a newline and exits 0. AC3.
func TestNoArgs_PrintsDev(t *testing.T) {
	t.Parallel()
	bin := testBinary(t)
	cmd := exec.Command(bin)
	out, err := cmd.Output()
	require.NoError(t, err, "binary should exit 0")
	assert.Equal(t, "dev\n", string(out))
}

// TestInjectedVersion verifies that a build with ldflags prints the injected
// version string. AC2.
func TestInjectedVersion(t *testing.T) {
	t.Parallel()
	bin := testBinaryWithVersion(t, "v1.2.3")
	cmd := exec.Command(bin)
	out, err := cmd.Output()
	require.NoError(t, err, "binary should exit 0")
	assert.Equal(t, "v1.2.3\n", string(out))
}

// TestVersionFlag verifies that --version prints the same output as no args. AC2.
func TestVersionFlag(t *testing.T) {
	t.Parallel()
	bin := testBinary(t)

	noArgs := exec.Command(bin)
	noArgsOut, err := noArgs.Output()
	require.NoError(t, err)

	withFlag := exec.Command(bin, "--version")
	flagOut, err := withFlag.Output()
	require.NoError(t, err)

	assert.Equal(t, string(noArgsOut), string(flagOut))
}

// TestVFlag verifies that -v prints the same output as no args. AC2.
func TestVFlag(t *testing.T) {
	t.Parallel()
	bin := testBinary(t)

	noArgs := exec.Command(bin)
	noArgsOut, err := noArgs.Output()
	require.NoError(t, err)

	withFlag := exec.Command(bin, "-v")
	flagOut, err := withFlag.Output()
	require.NoError(t, err)

	assert.Equal(t, string(noArgsOut), string(flagOut))
}

// TestUnknownFlag verifies that an unknown flag prints usage to stderr and
// exits 2. prd059-version R1.4.
func TestUnknownFlag(t *testing.T) {
	t.Parallel()
	bin := testBinary(t)
	cmd := exec.Command(bin, "--bogus")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	require.Error(t, err)
	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok, "expected ExitError")
	assert.Equal(t, 2, exitErr.ExitCode())
	assert.Contains(t, strings.ToLower(stderr.String()), "usage")
}

// TestVersionExported verifies that the Version() function returns the
// default version string. R1.5.
func TestVersionExported(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "dev", Version())
}
