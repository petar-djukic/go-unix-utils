// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd012-yes R1.1 (print string + newline in infinite loop),
// R1.2 (join multiple args with space), R1.3 (default to "y"), R1.4 (SIGPIPE handling).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R1.4, ARCHITECTURE shared_protocols: exit 0 on SIGPIPE.
	sys.InstallSIGPIPEHandler()

	// R1.2, R1.3: default to "y"; join multiple args with a single space.
	output := "y"
	if len(os.Args) > 1 {
		output = strings.Join(os.Args[1:], " ")
	}

	// R1.1: print STRING followed by a newline in an infinite loop.
	for {
		if _, err := fmt.Println(output); err != nil {
			os.Exit(1)
		}
	}
}
