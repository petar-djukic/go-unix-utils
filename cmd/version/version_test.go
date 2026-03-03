// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

// TestRun_NoArgs verifies that invocation with no arguments prints the version
// string followed by a newline and exits 0. (prd011-magefiles R1.1; AC4)
func TestRun_NoArgs(t *testing.T) {
	stdout, stderr, code := run(nil)
	if stdout != version+"\n" {
		t.Errorf("stdout = %q, want %q", stdout, version+"\n")
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

// TestRun_VersionFlags verifies that --version and -v each print the same
// output as no-argument invocation. (prd011-magefiles R1.4; AC2)
func TestRun_VersionFlags(t *testing.T) {
	for _, flag := range []string{"--version", "-v"} {
		t.Run(flag, func(t *testing.T) {
			stdout, stderr, code := run([]string{flag})
			if stdout != version+"\n" {
				t.Errorf("run(%q) stdout = %q, want %q", flag, stdout, version+"\n")
			}
			if stderr != "" {
				t.Errorf("run(%q) stderr = %q, want empty", flag, stderr)
			}
			if code != 0 {
				t.Errorf("run(%q) exit code = %d, want 0", flag, code)
			}
		})
	}
}

// TestRun_DefaultVersion verifies that the version string defaults to "dev"
// when not set via ldflags, as required for development builds.
// (prd011-magefiles R1.2; AC4)
func TestRun_DefaultVersion(t *testing.T) {
	if version != "dev" {
		t.Skipf("version = %q (set via ldflags); skipping dev-default check", version)
	}
	stdout, stderr, code := run(nil)
	if stdout != "dev\n" {
		t.Errorf("stdout = %q, want %q", stdout, "dev\n")
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

// TestRun_UnknownFlag verifies that an unrecognized flag causes a non-empty
// usage message on stderr, empty stdout, and exits 2.
// (prd011-magefiles R1.4; AC3)
func TestRun_UnknownFlag(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"bogus flag", []string{"--bogus"}},
		{"help flag", []string{"--help"}},
		{"extra positional", []string{"extra"}},
		{"version plus bogus", []string{"--version", "--bogus"}},
		{"multiple args", []string{"-v", "extra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := run(tc.args)
			if stdout != "" {
				t.Errorf("run(%v) stdout = %q, want empty", tc.args, stdout)
			}
			if stderr == "" {
				t.Errorf("run(%v) stderr is empty, want usage message", tc.args)
			}
			if !strings.Contains(stderr, "usage") {
				t.Errorf("run(%v) stderr = %q, want it to contain \"usage\"", tc.args, stderr)
			}
			if code != 2 {
				t.Errorf("run(%v) exit code = %d, want 2", tc.args, code)
			}
		})
	}
}

// TestRun_OutputMatchesNoArgs verifies that --version output is byte-for-byte
// identical to no-argument output. (prd011-magefiles R1.4; AC2)
func TestRun_OutputMatchesNoArgs(t *testing.T) {
	noArgOut, _, _ := run(nil)
	flagOut, _, _ := run([]string{"--version"})
	if noArgOut != flagOut {
		t.Errorf("--version output %q differs from no-arg output %q", flagOut, noArgOut)
	}
	shortFlagOut, _, _ := run([]string{"-v"})
	if noArgOut != shortFlagOut {
		t.Errorf("-v output %q differs from no-arg output %q", shortFlagOut, noArgOut)
	}
}
