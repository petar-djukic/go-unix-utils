// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package encutil provides shared encoding/decoding logic for cmd/ utilities
// that perform base32, base64, and other encoding operations.
//
// Implements prd088-encutil: R1 (encoder configuration), R2 (decoder
// configuration), R3 (encode operation), R4 (decode and input operations).
package encutil

import (
	"io"
	"os"
)

// EncoderConfig parameterizes an encoding operation with the encoding
// function and line-wrap column width.
//
// R1.1: EncoderConfig struct definition.
type EncoderConfig struct {
	Encode  func([]byte) string // encoding function (e.g. base64.StdEncoding.EncodeToString)
	WrapCol int                 // wrap output at this column width (0 = no wrap)
}

// DecoderConfig parameterizes a decoding operation with the decoding
// function and garbage-handling flag.
//
// R2.1: DecoderConfig struct definition.
type DecoderConfig struct {
	Decode        func(string) ([]byte, error) // decoding function
	IgnoreGarbage bool                         // skip non-alphabet characters
}

// Encode reads from r, encodes using cfg, and writes the result to w.
//
// R3.1: Encode function stub.
func Encode(r io.Reader, w io.Writer, cfg EncoderConfig) error {
	return nil
}

// Decode reads from r, decodes using cfg, and writes the result to w.
//
// R4.1: Decode function stub.
func Decode(r io.Reader, w io.Writer, cfg DecoderConfig) error {
	return nil
}

// OpenInput opens a file for reading, or returns os.Stdin if filename is "-".
//
// R4.2: OpenInput function stub.
func OpenInput(filename string) (io.ReadCloser, error) {
	if filename == "-" {
		return os.Stdin, nil
	}
	return os.Open(filename)
}
