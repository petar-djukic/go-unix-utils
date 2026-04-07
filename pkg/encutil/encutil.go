// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package encutil provides shared encoding/decoding logic used by cmd/
// encoding utilities (base32, base64, basenc).
//
// Implements: srd088-encutil R1 (EncoderConfig, Encode), R2 (DecoderConfig,
// Decode), R3 (OpenInput).
package encutil

import (
	"io"
	"os"
	"strings"
)

// EncoderConfig parameterizes an encoding operation with the encoding
// function and line-wrapping column width.
// R1.1: Encode is the encoding function; WrapCol is the column width for
// line wrapping (0 means no wrapping).
type EncoderConfig struct {
	Encode  func([]byte) string
	WrapCol int
}

// DecoderConfig parameterizes a decoding operation with the decoding
// function and garbage-tolerance flag.
// R2.1: Decode is the decoding function; IgnoreGarbage controls whether
// non-alphabet characters are silently skipped.
type DecoderConfig struct {
	Decode        func(string) ([]byte, error)
	IgnoreGarbage bool
}

// Encode reads from r, encodes the data using cfg, and writes the
// encoded output to w. Output is wrapped at cfg.WrapCol characters
// per line. When WrapCol is 0, the entire encoded string is written
// as a single line. A trailing newline is always appended.
// R1.2, R1.3: full encode pipeline with wrapping.
func Encode(r io.Reader, w io.Writer, cfg EncoderConfig) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	encoded := cfg.Encode(data)
	if cfg.WrapCol > 0 {
		if err := writeWrapped(w, encoded, cfg.WrapCol); err != nil {
			return err
		}
	} else {
		if _, err := io.WriteString(w, encoded); err != nil {
			return err
		}
	}
	_, err = io.WriteString(w, "\n")
	return err
}

// writeWrapped writes s to w, inserting a newline every wrapCol characters.
func writeWrapped(w io.Writer, s string, wrapCol int) error {
	for len(s) > wrapCol {
		if _, err := io.WriteString(w, s[:wrapCol]); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
		s = s[wrapCol:]
	}
	if len(s) > 0 {
		if _, err := io.WriteString(w, s); err != nil {
			return err
		}
	}
	return nil
}

// Decode reads from r, decodes the data using cfg, and writes the
// decoded output to w. Newlines and carriage returns are stripped
// before decoding. When IgnoreGarbage is true, all non-alphabet
// characters (outside A-Z, a-z, 0-9, +, /, =) are also stripped.
// When IgnoreGarbage is false, invalid characters cause the caller's
// Decode function to return an error.
// R2.1, R2.2, R2.3: full decode pipeline with whitespace stripping
// and garbage handling.
func Decode(r io.Reader, w io.Writer, cfg DecoderConfig) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	cleaned := stripWhitespace(string(data))
	if cfg.IgnoreGarbage {
		cleaned = stripGarbage(cleaned)
	}
	decoded, err := cfg.Decode(cleaned)
	if err != nil {
		return err
	}
	_, err = w.Write(decoded)
	return err
}

// stripWhitespace removes \n and \r from s.
func stripWhitespace(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// stripGarbage removes all characters that are not part of a base
// encoding alphabet (A-Z, a-z, 0-9, +, /, =) from s.
// R2.2: when IgnoreGarbage is true, non-alphabet characters are stripped.
func stripGarbage(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if isBaseAlphabet(s[i]) {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// isBaseAlphabet reports whether c is a character that could appear in
// a base encoding alphabet (A-Z, a-z, 0-9, +, /, =).
func isBaseAlphabet(c byte) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '+' || c == '/' || c == '='
}

// OpenInput opens a file for reading, or returns os.Stdin wrapped in a
// no-op ReadCloser when filename is "-" or empty.
// R3.1: stdin for "-" or empty; file handle otherwise.
func OpenInput(filename string) (io.ReadCloser, error) {
	if filename == "-" || filename == "" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(filename)
}
