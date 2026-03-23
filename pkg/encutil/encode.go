// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package encutil provides shared encoding and decoding operations for
// base32, base64, and basenc utilities.
//
// Implements prd088-encutil R2.1–R2.3, R3.1–R3.2: EncoderConfig,
// DecoderConfig structs and Encode, Decode, OpenInput function signatures
// (contract stubs).
package encutil

import (
	"io"
	"os"
)

// EncoderConfig holds the configuration for encoding operations.
//
// R2.1: Encode is the encoding function, WrapCol is the column at which
// to wrap output lines.
type EncoderConfig struct {
	Encode  func([]byte) string
	WrapCol int
}

// DecoderConfig holds the configuration for decoding operations.
//
// R2.2: Decode is the decoding function, IgnoreGarbage controls whether
// non-alphabet characters are silently skipped.
type DecoderConfig struct {
	Decode        func(string) ([]byte, error)
	IgnoreGarbage bool
}

// Encode reads from r, encodes the data using cfg, and writes to w.
//
// R2.3: stub — returns nil until the execution logic is implemented.
func Encode(r io.Reader, w io.Writer, cfg EncoderConfig) error {
	return nil
}

// Decode reads from r, decodes the data using cfg, and writes to w.
//
// R2.3: stub — returns nil until the execution logic is implemented.
func Decode(r io.Reader, w io.Writer, cfg DecoderConfig) error {
	return nil
}

// OpenInput opens the named file for reading, or returns os.Stdin if
// filename is "-".
//
// R3.1–R3.2: stub — returns nil, nil until the execution logic is
// implemented.
func OpenInput(filename string) (io.ReadCloser, error) {
	_ = os.Stdin // reference os to satisfy import constraint documentation
	return nil, nil
}
