// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package encutil provides shared encoding/decoding logic used by cmd/
// encoding utilities (base32, base64, basenc).
//
// Implements: srd088-encutil R1 (EncoderConfig), R2 (DecoderConfig),
// R3 (Encode, Decode, OpenInput function signatures).
package encutil

import (
	"io"
	"os"
)

// EncoderConfig parameterizes an encoding operation with the encoding
// function and line-wrapping column width.
// R1: Encode is the encoding function; WrapCol is the column width for
// line wrapping (0 means no wrapping).
type EncoderConfig struct {
	Encode  func([]byte) string
	WrapCol int
}

// DecoderConfig parameterizes a decoding operation with the decoding
// function and garbage-tolerance flag.
// R2: Decode is the decoding function; IgnoreGarbage controls whether
// non-alphabet characters are silently skipped.
type DecoderConfig struct {
	Decode        func(string) ([]byte, error)
	IgnoreGarbage bool
}

// Encode reads from r, encodes the data using cfg, and writes the
// encoded output to w.
// R3: stub — panics until implemented.
func Encode(r io.Reader, w io.Writer, cfg EncoderConfig) error {
	panic("not implemented")
}

// Decode reads from r, decodes the data using cfg, and writes the
// decoded output to w.
// R3: stub — panics until implemented.
func Decode(r io.Reader, w io.Writer, cfg DecoderConfig) error {
	panic("not implemented")
}

// OpenInput opens a file for reading, or returns os.Stdin wrapped in a
// no-op ReadCloser when filename is "-".
// R3: stub — panics until implemented.
func OpenInput(filename string) (io.ReadCloser, error) {
	_ = os.Stdin // reference os to satisfy import
	panic("not implemented")
}
