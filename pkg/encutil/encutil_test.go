// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for pkg/encutil: srd088 R1 (Encode), R2 (Decode), R3 (OpenInput).
package encutil

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// b64Encode is a test helper that wraps base64.StdEncoding.EncodeToString.
func b64Encode(src []byte) string {
	return base64.StdEncoding.EncodeToString(src)
}

// b64Decode is a test helper that wraps base64.StdEncoding.DecodeString.
func b64Decode(src string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(src)
}

// TestEncode verifies the Encode pipeline with wrapping and no-wrap modes.
// Traces: srd088 R1.1, R1.2, R1.3 (AC1, AC2).
func TestEncode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wrapCol int
		want    string
	}{
		{
			name:    "wrap at 76 characters",
			input:   strings.Repeat("A", 60),
			wrapCol: 76,
			want:    wrapString(b64Encode([]byte(strings.Repeat("A", 60))), 76) + "\n",
		},
		{
			name:    "long input wraps at 76",
			input:   strings.Repeat("B", 100),
			wrapCol: 76,
			want:    wrapString(b64Encode([]byte(strings.Repeat("B", 100))), 76) + "\n",
		},
		{
			name:    "no wrap when WrapCol is 0",
			input:   strings.Repeat("C", 200),
			wrapCol: 0,
			want:    b64Encode([]byte(strings.Repeat("C", 200))) + "\n",
		},
		{
			name:    "empty input",
			input:   "",
			wrapCol: 76,
			want:    "\n",
		},
		{
			name:    "wrap at 4 characters",
			input:   "Hello",
			wrapCol: 4,
			want:    wrapString(b64Encode([]byte("Hello")), 4) + "\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := strings.NewReader(tc.input)
			var w bytes.Buffer
			cfg := EncoderConfig{Encode: b64Encode, WrapCol: tc.wrapCol}
			err := Encode(r, &w, cfg)
			if err != nil {
				t.Fatalf("Encode returned error: %v", err)
			}
			if got := w.String(); got != tc.want {
				t.Errorf("Encode output mismatch\ngot:  %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// wrapString is a test helper that inserts newlines every n characters.
func wrapString(s string, n int) string {
	var b strings.Builder
	for len(s) > n {
		b.WriteString(s[:n])
		b.WriteByte('\n')
		s = s[n:]
	}
	b.WriteString(s)
	return b.String()
}

// TestDecode verifies the Decode pipeline with garbage handling.
// Traces: srd088 R2.1, R2.2, R2.3 (AC3, AC4).
func TestDecode(t *testing.T) {
	t.Parallel()

	encoded := b64Encode([]byte("Hello, World!"))

	tests := []struct {
		name          string
		input         string
		ignoreGarbage bool
		want          string
		wantErr       bool
	}{
		{
			name:          "valid base64 decodes",
			input:         encoded + "\n",
			ignoreGarbage: false,
			want:          "Hello, World!",
		},
		{
			name:          "strips newlines and carriage returns",
			input:         encoded[:4] + "\r\n" + encoded[4:] + "\n",
			ignoreGarbage: false,
			want:          "Hello, World!",
		},
		{
			name:          "ignore garbage strips non-alphabet chars",
			input:         encoded[:4] + "!@#$" + encoded[4:] + "\n",
			ignoreGarbage: true,
			want:          "Hello, World!",
		},
		{
			name:          "garbage causes error when not ignored",
			input:         encoded[:4] + "!@#$" + encoded[4:],
			ignoreGarbage: false,
			wantErr:       true,
		},
		{
			name:          "empty input decodes to empty",
			input:         "",
			ignoreGarbage: false,
			want:          "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := strings.NewReader(tc.input)
			var w bytes.Buffer
			cfg := DecoderConfig{Decode: b64Decode, IgnoreGarbage: tc.ignoreGarbage}
			err := Decode(r, &w, cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Decode returned error: %v", err)
			}
			if got := w.String(); got != tc.want {
				t.Errorf("Decode output mismatch\ngot:  %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// TestOpenInput verifies file handling and stdin fallback.
// Traces: srd088 R3.1, R3.2 (AC5).
func TestOpenInput(t *testing.T) {
	t.Parallel()

	t.Run("dash returns stdin", func(t *testing.T) {
		t.Parallel()
		rc, err := OpenInput("-")
		if err != nil {
			t.Fatalf("OpenInput('-') returned error: %v", err)
		}
		defer rc.Close() // best-effort close of NopCloser
		// R3.1: verify we get a reader wrapping os.Stdin
		if rc == nil {
			t.Fatal("OpenInput('-') returned nil")
		}
	})

	t.Run("empty string returns stdin", func(t *testing.T) {
		t.Parallel()
		rc, err := OpenInput("")
		if err != nil {
			t.Fatalf("OpenInput('') returned error: %v", err)
		}
		defer rc.Close()
		if rc == nil {
			t.Fatal("OpenInput('') returned nil")
		}
	})

	t.Run("existing file opens successfully", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		path := tmp + "/testfile"
		if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
			t.Fatalf("writing temp file: %v", err)
		}
		rc, err := OpenInput(path)
		if err != nil {
			t.Fatalf("OpenInput(%q) returned error: %v", path, err)
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("reading opened file: %v", err)
		}
		if string(data) != "content" {
			t.Errorf("got %q, want %q", string(data), "content")
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		t.Parallel()
		// R3.2: must return error when file cannot be opened
		_, err := OpenInput("/nonexistent/path/to/file")
		if err == nil {
			t.Fatal("expected error for nonexistent file, got nil")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("expected os.ErrNotExist, got: %v", err)
		}
	})
}
