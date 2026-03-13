// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd014-false R1.1–R1.3
package main

import (
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	// R1.2: all arguments are ignored.
	// R1.3: no reads from stdin, no writes to stdout or stderr.
	// R1.1: exit 1 unconditionally.
	os.Exit(1)
}
