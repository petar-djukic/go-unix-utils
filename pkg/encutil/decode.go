// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package encutil

import "io"

// DecoderConfig parameterizes a decoding operation with the decoding function
// and garbage-handling behavior.
// R3.1: Decode converts an encoded string back to raw bytes.
// R3.2: IgnoreGarbage skips non-alphabet characters in the input.
type DecoderConfig struct {
	Decode        func(string) ([]byte, error)
	IgnoreGarbage bool
}

// Decode reads encoded input from r, decodes it using cfg, and writes the raw
// bytes to w.
// R4.1: reads lines, applies cfg.Decode, writes decoded bytes to w.
func Decode(r io.Reader, w io.Writer, cfg DecoderConfig) error {
	panic("not implemented")
}

// OpenInput opens a file for reading, or returns os.Stdin when filename is "-".
// R5.1: returns an io.ReadCloser for the named file or stdin.
func OpenInput(filename string) (io.ReadCloser, error) {
	panic("not implemented")
}
