// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd013-true R1.1–R1.3
package main

import (
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	// R1.1: exit 0 unconditionally.
	// R1.2: all arguments are ignored — os.Args is never inspected.
	// R1.3: no reads from stdin, no writes to stdout or stderr.
}
