// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package encutil provides shared encoding and decoding operations for
// base32, base64, and basenc utilities.
//
// Implements prd088-encutil R1.1–R1.3, R2.1–R2.3, R3.1–R3.2.
package encutil

import (
	"bufio"
	"io"
	"os"
	"strings"
)

// EncoderConfig holds the configuration for encoding operations.
//
// R1.1: Encode is the encoding function, WrapCol is the column at which
// to wrap output lines (0 means no wrapping).
type EncoderConfig struct {
	Encode  func([]byte) string
	WrapCol int
}

// DecoderConfig holds the configuration for decoding operations.
//
// R2.1: Decode is the decoding function, IgnoreGarbage controls whether
// lines that fail to decode are silently skipped.
type DecoderConfig struct {
	Decode        func(string) ([]byte, error)
	IgnoreGarbage bool
}

// Encode reads from r, encodes the data using cfg, wraps output at
// cfg.WrapCol characters per line, and writes to w.
//
// R1.2: Reads all input, encodes, wraps, writes.
// R1.3: When WrapCol is 0, writes entire output as single line + newline.
func Encode(r io.Reader, w io.Writer, cfg EncoderConfig) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	encoded := cfg.Encode(data)
	return writeWrapped(w, encoded, cfg.WrapCol)
}

// writeWrapped writes s to w, inserting newlines at wrapCol boundaries.
// If wrapCol <= 0, writes s as a single line with a trailing newline.
func writeWrapped(w io.Writer, s string, wrapCol int) error {
	if wrapCol <= 0 {
		_, err := io.WriteString(w, s+"\n")
		return err
	}
	if len(s) == 0 {
		_, err := io.WriteString(w, "\n")
		return err
	}
	for len(s) > 0 {
		end := wrapCol
		if end > len(s) {
			end = len(s)
		}
		if _, err := io.WriteString(w, s[:end]+"\n"); err != nil {
			return err
		}
		s = s[end:]
	}
	return nil
}

// Decode reads encoded lines from r, decodes using cfg, and writes
// raw bytes to w.
//
// R2.2: Strips whitespace from each line before decoding.
// R2.3: When IgnoreGarbage is true, silently skips lines that fail
// to decode instead of returning an error.
func Decode(r io.Reader, w io.Writer, cfg DecoderConfig) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		decoded, err := cfg.Decode(line)
		if err != nil {
			if cfg.IgnoreGarbage {
				continue
			}
			return err
		}
		if _, err := w.Write(decoded); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// OpenInput opens the named file for reading, or returns os.Stdin if
// filename is empty or "-".
//
// R3.1: Returns os.Stdin wrapped in a no-op closer for "" or "-".
// R3.2: Opens and returns the file for real paths.
func OpenInput(filename string) (io.ReadCloser, error) {
	if filename == "" || filename == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(filename)
}
