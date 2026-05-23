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
// R1.2: reads input, applies cfg.Encode.
// R1.3: wraps at cfg.WrapCol; 0 means single-line output with trailing newline.
func Encode(r io.Reader, w io.Writer, cfg EncoderConfig) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	encoded := cfg.Encode(data)
	if cfg.WrapCol <= 0 {
		_, err = io.WriteString(w, encoded)
		return err
	}
	return writeWrapped(w, encoded, cfg.WrapCol)
}

func writeWrapped(w io.Writer, s string, col int) error {
	for len(s) > col {
		if _, err := io.WriteString(w, s[:col]+"\n"); err != nil {
			return err
		}
		s = s[col:]
	}
	if len(s) > 0 {
		if _, err := io.WriteString(w, s+"\n"); err != nil {
			return err
		}
	}
	return nil
}
