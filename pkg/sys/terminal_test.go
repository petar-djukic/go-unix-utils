// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for terminal.go: IsTerminal and GetWinSize.
// Implements: prd002-sys (R1)
package sys

import (
	"os"
	"testing"
)

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		name string
		fd   func() (uintptr, func())
		want bool
	}{
		{
			name: "pipe write end returns false",
			fd: func() (uintptr, func()) {
				r, w, err := os.Pipe()
				if err != nil {
					t.Fatalf("os.Pipe: %v", err)
				}
				cleanup := func() {
					r.Close()
					w.Close()
				}
				return w.Fd(), cleanup
			},
			want: false,
		},
		{
			name: "pipe read end returns false",
			fd: func() (uintptr, func()) {
				r, w, err := os.Pipe()
				if err != nil {
					t.Fatalf("os.Pipe: %v", err)
				}
				cleanup := func() {
					r.Close()
					w.Close()
				}
				return r.Fd(), cleanup
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fd, cleanup := tc.fd()
			t.Cleanup(cleanup)

			got := IsTerminal(fd)
			if got != tc.want {
				t.Fatalf("IsTerminal(fd) = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGetWinSize(t *testing.T) {
	t.Run("pipe fd returns error", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe: %v", err)
		}
		defer r.Close()
		defer w.Close()

		_, err = GetWinSize(w.Fd())
		if err == nil {
			t.Fatal("GetWinSize(pipe) returned nil error, want non-nil")
		}
	})

	t.Run("pipe read end returns error", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe: %v", err)
		}
		defer r.Close()
		defer w.Close()

		_, err = GetWinSize(r.Fd())
		if err == nil {
			t.Fatal("GetWinSize(pipe read) returned nil error, want non-nil")
		}
	})
}
