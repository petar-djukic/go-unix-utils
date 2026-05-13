// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package encutil provides shared encoding and decoding logic for base32,
// base64, and basenc utilities. Implements srd088-encutil.
package encutil

import "io"

// EncoderConfig parameterizes an encoding operation with the encoding function
// and line-wrap column width.
// R1.1: Encode converts raw bytes to an encoded string.
// R1.2: WrapCol controls output line wrapping (0 means no wrapping).
type EncoderConfig struct {
	Encode  func([]byte) string
	WrapCol int
}

// Encode reads all input from r, encodes it using cfg, and writes the wrapped
// output to w.
// R2.1: reads input, applies cfg.Encode, wraps at cfg.WrapCol, writes to w.
func Encode(r io.Reader, w io.Writer, cfg EncoderConfig) error {
	panic("not implemented")
}
