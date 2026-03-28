// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package encutil_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/encutil"
)

func b64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func b64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func TestEncode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wrapCol int
		want    string
	}{
		{
			name:    "wrap at 4 chars",
			input:   "Hello World",
			wrapCol: 4,
			want:    "SGVs\nbG8g\nV29y\nbGQ=\n",
		},
		{
			name:    "no wrap when WrapCol is 0",
			input:   "Hello World",
			wrapCol: 0,
			want:    "SGVsbG8gV29ybGQ=\n",
		},
		{
			name:    "wrap at 76 default",
			input:   "short",
			wrapCol: 76,
			want:    "c2hvcnQ=\n",
		},
		{
			name:    "empty input",
			input:   "",
			wrapCol: 76,
			want:    "\n",
		},
		{
			name:    "exact wrap boundary",
			input:   "aaa",
			wrapCol: 4,
			want:    "YWFh\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := strings.NewReader(tc.input)
			var buf bytes.Buffer
			cfg := encutil.EncoderConfig{
				Encode:  b64Encode,
				WrapCol: tc.wrapCol,
			}
			err := encutil.Encode(r, &buf, cfg)
			if err != nil {
				t.Fatalf("Encode returned error: %v", err)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("Encode output mismatch\ngot:  %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestEncode_WrapAt76(t *testing.T) {
	t.Parallel()

	// R1.2: Generate input long enough to require wrapping at 76 columns.
	input := strings.Repeat("A", 100)
	r := strings.NewReader(input)
	var buf bytes.Buffer
	cfg := encutil.EncoderConfig{
		Encode:  b64Encode,
		WrapCol: 76,
	}
	err := encutil.Encode(r, &buf, cfg)
	if err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	for i, line := range lines {
		if i < len(lines)-1 && len(line) != 76 {
			t.Errorf("line %d: length %d, want 76", i, len(line))
		}
		if i == len(lines)-1 && len(line) > 76 {
			t.Errorf("last line: length %d, want <= 76", len(line))
		}
	}
}

func TestDecode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		ignoreGarbage bool
		want          string
		wantErr       bool
	}{
		{
			name:  "single line decode",
			input: "SGVsbG8gV29ybGQ=\n",
			want:  "Hello World",
		},
		{
			name:  "multi-line wrapped decode",
			input: "SGVs\nbG8g\nV29y\nbGQ=\n",
			want:  "Hello World",
		},
		{
			name:  "blank lines skipped",
			input: "\n  \nSGVsbG8=\n\n",
			want:  "Hello",
		},
		{
			name:          "ignore garbage skips bad lines",
			input:         "SGVsbG8=\n!!!invalid!!!\nV29ybGQ=\n",
			ignoreGarbage: true,
			want:          "HelloWorld",
		},
		{
			name:          "error on garbage when not ignored",
			input:         "SGVsbG8=\n!!!invalid!!!\n",
			ignoreGarbage: false,
			wantErr:       true,
		},
		{
			name:  "whitespace trimmed from lines",
			input: "  SGVsbG8=  \n",
			want:  "Hello",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := strings.NewReader(tc.input)
			var buf bytes.Buffer
			cfg := encutil.DecoderConfig{
				Decode:        b64Decode,
				IgnoreGarbage: tc.ignoreGarbage,
			}
			err := encutil.Decode(r, &buf, cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatal("Decode should have returned error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Decode returned error: %v", err)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("Decode output mismatch\ngot:  %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestOpenInput_Stdin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
	}{
		{name: "empty string", filename: ""},
		{name: "dash", filename: "-"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rc, err := encutil.OpenInput(tc.filename)
			if err != nil {
				t.Fatalf("OpenInput(%q) returned error: %v", tc.filename, err)
			}
			if rc == nil {
				t.Fatal("OpenInput returned nil ReadCloser")
			}
			// R3.1: Closing should not close actual stdin; no-op closer.
			if err := rc.Close(); err != nil {
				t.Errorf("Close returned error: %v", err)
			}
		})
	}
}

func TestOpenInput_File(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "test content\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	rc, err := encutil.OpenInput(path)
	if err != nil {
		t.Fatalf("OpenInput(%q) returned error: %v", path, err)
	}
	defer rc.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rc); err != nil {
		t.Fatalf("reading from OpenInput result: %v", err)
	}
	if got := buf.String(); got != content {
		t.Errorf("content mismatch\ngot:  %q\nwant: %q", got, content)
	}
}

func TestOpenInput_NonexistentFile(t *testing.T) {
	t.Parallel()

	_, err := encutil.OpenInput("/nonexistent/path/file.txt")
	if err == nil {
		t.Fatal("OpenInput should return error for nonexistent file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got: %v", err)
	}
}
