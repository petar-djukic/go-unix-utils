// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd007-sponge R1.1–R1.4
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R1.4: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	flag.Parse()
	args := flag.Args()

	// R1.1: accept zero or one positional FILE argument.
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "sponge: too many arguments\n")
		os.Exit(1)
	}

	// R1.2: read all of stdin into memory before any write or file operation.
	buf, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: read error: %v\n", err)
		os.Exit(1)
	}

	// R1.1, R1.3: if no FILE given, write to stdout; otherwise write to FILE.
	if len(args) == 0 {
		// R1.1: passthrough mode — write buffered stdin to stdout.
		if _, err := os.Stdout.Write(buf); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: write error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// R1.3: write complete buffer to FILE after stdin is exhausted.
	outPath := args[0]
	if err := os.WriteFile(outPath, buf, 0o666); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		os.Exit(1)
	}
}
