// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/sponge: soak up stdin and write to a file.
// Implements prd007-sponge R1-R4.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// D1: Install SIGPIPE handler per ARCHITECTURE.yaml shared_protocols.
	sys.InstallSIGPIPEHandler()

	appendMode, file := parseArgs(os.Args[1:])

	// R1: Read all of stdin into memory before writing any output.
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: read error: %v\n", err)
		os.Exit(1)
	}

	// R3: No filename argument means write to stdout.
	if file == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: write error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// R2: -a flag appends; otherwise overwrite.
	if appendMode {
		f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			os.Exit(1)
		}
		_, writeErr := f.Write(data)
		closeErr := f.Close()
		if writeErr != nil {
			fmt.Fprintf(os.Stderr, "sponge: write error: %v\n", writeErr)
			os.Exit(1)
		}
		if closeErr != nil {
			fmt.Fprintf(os.Stderr, "sponge: close error: %v\n", closeErr)
			os.Exit(1)
		}
	} else {
		if err := os.WriteFile(file, data, 0666); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			os.Exit(1)
		}
	}
}

// parseArgs parses sponge flags from args. Returns append mode and optional filename.
func parseArgs(args []string) (bool, string) {
	appendMode := false
	var files []string

	for _, arg := range args {
		if arg == "-a" {
			appendMode = true
		} else if arg == "--" {
			continue
		} else if len(arg) > 0 && arg[0] == '-' && arg != "-" {
			fmt.Fprintf(os.Stderr, "sponge: invalid option -- '%s'\n", arg[1:])
			os.Exit(1)
		} else {
			files = append(files, arg)
		}
	}

	if len(files) > 1 {
		fmt.Fprintf(os.Stderr, "sponge: too many arguments\n")
		os.Exit(1)
	}

	file := ""
	if len(files) == 1 {
		file = files[0]
	}
	return appendMode, file
}
