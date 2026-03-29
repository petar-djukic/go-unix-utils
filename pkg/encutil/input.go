// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// input.go implements prd088 R2.1: input file opening with stdin fallback.

package encutil

import (
	"io"
	"os"
)

// OpenInput opens a file for reading, or returns os.Stdin if filename is "-"
// or empty. When returning stdin, wraps it in io.NopCloser so the caller can
// Close unconditionally.
//
// R2.1: stdin fallback with safe-to-close wrapper.
func OpenInput(filename string) (io.ReadCloser, error) {
	if filename == "-" || filename == "" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(filename)
}
