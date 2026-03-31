// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// encode.go implements prd088 R1.1, R1.2, R1.3:
// encoding with configurable line wrapping.

package encutil

import "io"

// Encode reads all data from r, encodes it using cfg.Encode, and writes the
// encoded output to w with optional line wrapping.
//
// R1.1: reads all data and encodes via cfg.Encode.
// R1.2: wraps output at WrapCol characters when WrapCol > 0.
// R1.3: emits a single line followed by newline when WrapCol is 0.
func Encode(r io.Reader, w io.Writer, cfg EncoderConfig) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	encoded := cfg.Encode(data)
	if cfg.WrapCol > 0 {
		return writeWrapped(w, encoded, cfg.WrapCol)
	}
	_, err = io.WriteString(w, encoded+"\n")
	return err
}

// writeWrapped writes s to w, inserting a newline every wrapCol characters.
// R1.2: line wrapping at the configured column width.
func writeWrapped(w io.Writer, s string, wrapCol int) error {
	for len(s) > wrapCol {
		if _, err := io.WriteString(w, s[:wrapCol]+"\n"); err != nil {
			return err
		}
		s = s[wrapCol:]
	}
	if len(s) > 0 {
		if _, err := io.WriteString(w, s+"\n"); err != nil {
			return err
		}
	}
	return nil
}
