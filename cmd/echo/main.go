// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/echo implements GNU echo: display a line of text.
// Implements prd020-echo.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	// TODO R1: Core output behavior (R1.1-R1.4)
	// TODO R2: Escape sequence interpretation (R2.1-R2.4)
	// TODO R3: Exit codes and SIGPIPE (R3.1-R3.2)

	args := os.Args[1:]
	_, err := fmt.Fprintln(os.Stdout, strings.Join(args, " "))
	if err != nil {
		os.Exit(1)
	}
}
