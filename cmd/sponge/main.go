// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the sponge utility for soaking stdin and writing to a file.
//
// Implements prd007-sponge: core soak-before-write (R1), output file handling (R2),
// append mode (R3), passthrough mode (R4), exit codes and error handling (R5).
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	appendMode, filename := parseArgs(os.Args[1:])

	// R1.1: Read all bytes from stdin before opening the output file.
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: read error: %v\n", err)
		os.Exit(1)
	}

	// R4.1: When no output filename is given, write to stdout.
	if filename == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: write error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := writeFile(filename, data, appendMode); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		os.Exit(1)
	}
}

// parseArgs extracts the -a flag and output filename from command-line arguments.
func parseArgs(args []string) (appendMode bool, filename string) {
	for _, arg := range args {
		if arg == "--" {
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'a':
					appendMode = true
				default:
					fmt.Fprintf(os.Stderr, "sponge: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
		} else {
			filename = arg
		}
	}
	return appendMode, filename
}

// writeFile writes data to the named file, optionally appending.
func writeFile(filename string, data []byte, appendMode bool) error {
	// R2: Determine open flags.
	flags := os.O_WRONLY | os.O_CREATE
	if appendMode {
		// R3.1: -a appends stdin content to the existing file.
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	f, err := os.OpenFile(filename, flags, 0o644)
	if err != nil {
		return fmt.Errorf("%s: %w", filename, err)
	}

	if _, err := f.Write(data); err != nil {
		f.Close() // best-effort close on write failure
		return fmt.Errorf("write error: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("close error: %w", err)
	}

	return nil
}
