// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Unit tests for the version command.
//
// Implements: prd011-magefiles R1.1, R1.2, R1.4, R1.5
package main

import (
	"io"
	"os"
	"testing"
)

// captureOutput replaces os.Stdout and os.Stderr with pipes, calls fn, and
// returns whatever fn wrote to each stream.
func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe for stdout: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		outR.Close()
		outW.Close()
		t.Fatalf("os.Pipe for stderr: %v", err)
	}

	origOut := os.Stdout
	origErr := os.Stderr
	os.Stdout = outW
	os.Stderr = errW

	fn()

	outW.Close()
	errW.Close()
	os.Stdout = origOut
	os.Stderr = origErr

	outBytes, err := io.ReadAll(outR)
	outR.Close()
	if err != nil {
		t.Fatalf("reading stdout pipe: %v", err)
	}
	errBytes, err := io.ReadAll(errR)
	errR.Close()
	if err != nil {
		t.Fatalf("reading stderr pipe: %v", err)
	}

	return string(outBytes), string(errBytes)
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		setVersion string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		// prd011-magefiles R1.1: no arguments prints version string to stdout, exits 0.
		{
			name:       "no args prints version string",
			args:       []string{},
			setVersion: "v1.20260301.1",
			wantCode:   0,
			wantStdout: "v1.20260301.1\n",
		},
		// prd011-magefiles R1.2: default version is "dev" when linker variable is not set.
		{
			name:       "default version is dev",
			args:       []string{},
			wantCode:   0,
			wantStdout: "dev\n",
		},
		// prd011-magefiles R1.4: --version flag prints version string, exits 0.
		{
			name:       "--version flag prints version",
			args:       []string{"--version"},
			wantCode:   0,
			wantStdout: "dev\n",
		},
		// prd011-magefiles R1.4: -v flag prints version string, exits 0.
		{
			name:       "-v flag prints version",
			args:       []string{"-v"},
			wantCode:   0,
			wantStdout: "dev\n",
		},
		// prd011-magefiles R1.5: unrecognized argument prints usage to stderr, exits 2.
		{
			name:       "unrecognized arg prints usage to stderr",
			args:       []string{"--bogus"},
			wantCode:   2,
			wantStderr: "usage: version [--version | -v]\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origVersion := version
			if tt.setVersion != "" {
				version = tt.setVersion
			}
			defer func() { version = origVersion }()

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = run(tt.args)
			})

			if code != tt.wantCode {
				t.Errorf("run(%v) exit code = %d, want %d", tt.args, code, tt.wantCode)
			}
			if stdout != tt.wantStdout {
				t.Errorf("run(%v) stdout = %q, want %q", tt.args, stdout, tt.wantStdout)
			}
			if stderr != tt.wantStderr {
				t.Errorf("run(%v) stderr = %q, want %q", tt.args, stderr, tt.wantStderr)
			}
		})
	}
}
