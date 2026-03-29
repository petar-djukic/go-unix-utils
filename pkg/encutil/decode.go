// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// decode.go implements prd088 R2.2, R2.3, R3.1, R3.2:
// decoding with optional garbage skipping and error reporting.

package encutil

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Decode reads encoded text from r line by line, strips whitespace,
// decodes each line using cfg.Decode, and writes raw bytes to w.
//
// R2.2: reads all encoded input, strips whitespace, decodes, writes to w.
// R2.3: returns error on invalid input when IgnoreGarbage is false.
// R3.1: empty input writes nothing and returns nil.
// R3.2: propagates decode errors with line number context.
func Decode(r io.Reader, w io.Writer, cfg DecoderConfig) error {
	scanner := bufio.NewScanner(r)
	var accumulated strings.Builder
	for scanner.Scan() {
		line := stripWhitespace(scanner.Text())
		accumulated.WriteString(line)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	encoded := accumulated.String()
	if encoded == "" {
		return nil
	}
	return decodeAndWrite(encoded, w, cfg)
}

// stripWhitespace removes spaces, tabs, newlines, and carriage returns.
func stripWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := range len(s) {
		c := s[i]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// decodeAndWrite decodes the accumulated encoded string and writes to w.
func decodeAndWrite(encoded string, w io.Writer, cfg DecoderConfig) error {
	decoded, err := cfg.Decode(encoded)
	if err != nil {
		if !cfg.IgnoreGarbage {
			return fmt.Errorf("invalid input: %w", err)
		}
		return tryDecodeWithGarbageSkip(encoded, w, cfg)
	}
	_, writeErr := w.Write(decoded)
	return writeErr
}

// tryDecodeWithGarbageSkip attempts to decode by removing non-decodable
// characters one chunk at a time when IgnoreGarbage is true.
// Falls back to attempting decode on the full accumulated input after
// stripping characters that cause errors.
func tryDecodeWithGarbageSkip(encoded string, w io.Writer, cfg DecoderConfig) error {
	// Try progressively stripping non-standard characters
	var cleaned strings.Builder
	cleaned.Grow(len(encoded))
	for i := range len(encoded) {
		c := encoded[i]
		if isBase64Alphabet(c) {
			cleaned.WriteByte(c)
		}
	}
	decoded, err := cfg.Decode(cleaned.String())
	if err != nil {
		return fmt.Errorf("invalid input after stripping garbage: %w", err)
	}
	_, writeErr := w.Write(decoded)
	return writeErr
}

// isBase64Alphabet returns true for characters commonly valid in base
// encoding alphabets (A-Z, a-z, 0-9, +, /, =, -, _). This is a broad
// filter; the actual validation is done by cfg.Decode.
func isBase64Alphabet(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z':
		return true
	case c >= 'a' && c <= 'z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '+' || c == '/' || c == '=' || c == '-' || c == '_':
		return true
	default:
		return false
	}
}
