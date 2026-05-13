// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package encutil

import (
	"io"
	"os"
	"strings"
)

// DecoderConfig parameterizes a decoding operation with the decoding function
// and garbage-handling behavior.
// R2.1: Decode converts an encoded string back to raw bytes.
// R2.1: IgnoreGarbage skips non-alphabet characters in the input.
type DecoderConfig struct {
	Decode        func(string) ([]byte, error)
	IgnoreGarbage bool
}

// Decode reads encoded input from r, decodes it using cfg, and writes the raw
// bytes to w.
// R2.2: reads all input, strips whitespace, decodes via cfg.Decode, writes to w.
// R2.3: returns error on invalid characters when IgnoreGarbage is false.
func Decode(r io.Reader, w io.Writer, cfg DecoderConfig) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	input := stripWhitespace(string(data))
	if cfg.IgnoreGarbage {
		input = stripGarbage(input)
	}
	decoded, err := cfg.Decode(input)
	if err != nil {
		return err
	}
	_, err = w.Write(decoded)
	return err
}

func stripWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// stripGarbage removes characters not in the common base-encoding superset
// alphabet (A-Za-z0-9+/=-_), covering Base64, Base64URL, Base32, Base32hex,
// and Base16.
func stripGarbage(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if isEncodingAlphabet(s[i]) {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func isEncodingAlphabet(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '+' || ch == '/' || ch == '=' ||
		ch == '-' || ch == '_'
}

// OpenInput opens a file for reading, or returns os.Stdin when filename is
// empty or "-".
// R3.1: returns an io.ReadCloser for the named file or stdin.
// R3.2: returns an error when the file cannot be opened.
func OpenInput(filename string) (io.ReadCloser, error) {
	if filename == "" || filename == "-" {
		return os.Stdin, nil
	}
	return os.Open(filename)
}
