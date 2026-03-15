// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"testing"
)

func TestTempDirNormalizer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "empty input",
			input: []byte{},
			want:  []byte{},
		},
		{
			name:  "no temp paths",
			input: []byte("hello world\n"),
			want:  []byte("hello world\n"),
		},
		{
			name:  "linux tmp path",
			input: []byte("/tmp/test.XXXXXX\n"),
			want:  []byte("<TEMPDIR>\n"),
		},
		{
			name:  "macos var folders path",
			input: []byte("/var/folders/dl/abc123/T/test.XXXXXX\n"),
			want:  []byte("<TEMPDIR>\n"),
		},
		{
			name:  "macos private tmp path",
			input: []byte("/private/tmp/test.XXXXXX\n"),
			want:  []byte("<TEMPDIR>\n"),
		},
		{
			name:  "macos private var folders path",
			input: []byte("/private/var/folders/dl/abc123/T/test.XXXXXX\n"),
			want:  []byte("<TEMPDIR>\n"),
		},
		{
			name:  "multiple temp paths",
			input: []byte("/tmp/file1\n/tmp/file2\n"),
			want:  []byte("<TEMPDIR>\n<TEMPDIR>\n"),
		},
		{
			name:  "temp path in middle of line",
			input: []byte("created /tmp/test.abc123 successfully\n"),
			want:  []byte("created <TEMPDIR> successfully\n"),
		},
		{
			name:  "non-temp path unchanged",
			input: []byte("/home/user/file.txt\n"),
			want:  []byte("/home/user/file.txt\n"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := TempDirNormalizer(tc.input)
			if !bytes.Equal(got, tc.want) {
				t.Errorf("TempDirNormalizer(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestErrorPrefixNormalizer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "empty input",
			input: []byte{},
			want:  []byte{},
		},
		{
			name:  "no error prefix",
			input: []byte("hello world\n"),
			want:  []byte("hello world\n"),
		},
		{
			name:  "go binary error",
			input: []byte("mktemp: cannot create temp file\n"),
			want:  []byte("<PROG>: cannot create temp file\n"),
		},
		{
			name:  "gnu reference binary error",
			input: []byte("gmktemp: cannot create temp file\n"),
			want:  []byte("<PROG>: cannot create temp file\n"),
		},
		{
			name:  "multiple error lines",
			input: []byte("mkdir: cannot create directory 'a'\nmkdir: cannot create directory 'b'\n"),
			want:  []byte("<PROG>: cannot create directory 'a'\n<PROG>: cannot create directory 'b'\n"),
		},
		{
			name:  "mixed error and normal lines",
			input: []byte("some output\nln: failed to create link\nmore output\n"),
			want:  []byte("some output\n<PROG>: failed to create link\nmore output\n"),
		},
		{
			name:  "binary with digits",
			input: []byte("sha256sum: file.txt: No such file\n"),
			want:  []byte("<PROG>: file.txt: No such file\n"),
		},
		{
			name:  "gnu prefixed binary with digits",
			input: []byte("gsha256sum: file.txt: No such file\n"),
			want:  []byte("<PROG>: file.txt: No such file\n"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ErrorPrefixNormalizer(tc.input)
			if !bytes.Equal(got, tc.want) {
				t.Errorf("ErrorPrefixNormalizer(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
