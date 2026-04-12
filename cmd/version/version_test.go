// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/version.
// Implements srd059-version R1.1, R1.2, R1.4, R1.5 and srd011-magefiles R5.1.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs the standard differential test pattern. There is no GNU
// reference binary for version, so this always skips gracefully.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gversion")
	if err != nil {
		t.Skip("reference binary gversion not in PATH")
	}
	tests := []testutils.DiffTest{
		{
			Name: "no_args",
			Args: nil,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestNoArgs verifies R1.1: no arguments prints version string followed
// by a newline and exits 0. R1.2: without ldflags, version is "dev".
func TestNoArgs(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("version with no args failed: %v", err)
	}
	got := string(out)
	if got != "dev\n" {
		t.Errorf("no-args output = %q, want %q", got, "dev\n")
	}
}

// TestVersionFlag verifies R1.4: --version prints the same output as
// no-argument invocation.
func TestVersionFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	for _, flag := range []string{"--version", "-v"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(goBin, flag)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("version %s failed: %v", flag, err)
			}
			got := string(out)
			if got != "dev\n" {
				t.Errorf("version %s output = %q, want %q", flag, got, "dev\n")
			}
		})
	}
}

// TestUnknownFlag verifies R1.4: any other flag prints usage to stderr
// and exits 2.
func TestUnknownFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "--bogus")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for --bogus, got nil error")
	}
	var exitErr *exec.ExitError
	if ok := isExitError(err, &exitErr); !ok {
		t.Fatalf("unexpected error type: %v", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2", exitErr.ExitCode())
	}
	if !strings.Contains(string(out), "Usage:") {
		t.Errorf("stderr should contain Usage message, got %q", string(out))
	}
}

// TestExportedVersion verifies R1.5: the Version variable is exported and
// defaults to "dev" when ldflags are not set.
func TestExportedVersion(t *testing.T) {
	t.Parallel()
	if Version != "dev" {
		t.Errorf("Version = %q, want %q", Version, "dev")
	}
}

// TestGetVersion verifies R1.5: GetVersion returns the current version string.
func TestGetVersion(t *testing.T) {
	t.Parallel()
	got := GetVersion()
	if got != "dev" {
		t.Errorf("GetVersion() = %q, want %q", got, "dev")
	}
}

// TestLdflagsInjection verifies R1.5: the version can be set at build time
// via -ldflags so other cmd/ packages can use the same mechanism.
func TestLdflagsInjection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	binPath := filepath.Join(dir, "version")
	const want = "v42.0.0-test"

	build := exec.Command("go", "build",
		"-ldflags", "-X main.Version="+want,
		"-o", binPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build with ldflags failed: %v\n%s", err, out)
	}

	run := exec.Command(binPath)
	out, err := run.Output()
	if err != nil {
		t.Fatalf("version binary failed: %v", err)
	}
	got := strings.TrimSpace(string(out))
	if got != want {
		t.Errorf("version with ldflags = %q, want %q", got, want)
	}
}

// TestConfigurationFields verifies srd011 R5.1: configuration.yaml project
// section contains module_path, go_source_dirs, and magefiles_dir fields.
// binary_dir may be defaulted by the scaffold when not explicitly set.
func TestConfigurationFields(t *testing.T) {
	t.Parallel()
	configPath := filepath.Join("..", "..", "configuration.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Skipf("configuration.yaml not found: %v", err)
	}

	content := string(data)
	// R5.1: module_path, go_source_dirs, magefiles_dir must be present.
	// binary_dir may be scaffold-defaulted and not explicitly listed.
	required := []string{"module_path", "go_source_dirs", "magefiles_dir"}
	for _, field := range required {
		if !strings.Contains(content, field) {
			t.Errorf("configuration.yaml missing required field %q", field)
		}
	}
}

// isExitError checks if err is an *exec.ExitError and assigns it to target.
func isExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}
