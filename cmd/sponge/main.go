// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd007-sponge R1.1–R1.4 (core stdin reading and temporary file
// buffering). Reads all of stdin into a temporary file before writing to the
// output file or stdout.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R5.4: exit 0 on SIGPIPE when stdout is closed by a downstream consumer.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	var outFile string
	if len(args) > 0 {
		outFile = args[len(args)-1]
	}

	// R1.4: create temp file in the same directory as the output file
	// to ensure atomic rename works on the same filesystem. When no
	// filename is given, use the system temp directory.
	tmpDir := os.TempDir()
	if outFile != "" {
		tmpDir = filepath.Dir(outFile)
	}

	// R1.1: read all of stdin into a temporary file before opening or
	// creating the output file.
	tmp, err := os.CreateTemp(tmpDir, "sponge.*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: failed to create temporary file: %v\n", err)
		os.Exit(1)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // best-effort cleanup

	if _, err := io.Copy(tmp, os.Stdin); err != nil {
		tmp.Close() // best-effort close before exit
		fmt.Fprintf(os.Stderr, "sponge: error reading stdin: %v\n", err)
		os.Exit(1)
	}

	if err := tmp.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: error closing temporary file: %v\n", err)
		os.Exit(1)
	}

	if outFile == "" {
		// R1.3: no filename argument — write buffered stdin to stdout.
		if err := writeToStdout(tmpName); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// R1.2: write buffered stdin to the named output file via atomic rename.
	if err := os.Rename(tmpName, outFile); err != nil {
		// Cross-device fallback: copy content then remove temp file.
		if err := copyFile(tmpName, outFile); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			os.Exit(1)
		}
	}
}

// writeToStdout opens the temporary file and copies its content to stdout.
func writeToStdout(tmpName string) error {
	f, err := os.Open(tmpName)
	if err != nil {
		return fmt.Errorf("failed to reopen temporary file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(os.Stdout, f); err != nil {
		return fmt.Errorf("write error: %w", err)
	}
	return nil
}

// copyFile copies the content of src to dst, creating or truncating dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open temporary file for copy: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close() // best-effort close before returning error
		return fmt.Errorf("failed to copy to output file: %w", err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("failed to close output file: %w", err)
	}
	return nil
}
