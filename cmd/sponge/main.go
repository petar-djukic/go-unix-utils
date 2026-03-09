// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/sponge: soak stdin and write to file.
// Implements: prd007-sponge (R1–R5).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// usage is the sponge usage message.
const usage = "Usage: sponge [-a] [file]\n"

func main() {
	// D1: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	append, file := parseArgs(os.Args[1:])
	exitCode := run(append, file)
	os.Exit(exitCode)
}

// parseArgs parses sponge flags and returns (appendMode, outputFile).
// outputFile is empty when no file argument is given (passthrough mode).
func parseArgs(args []string) (bool, string) {
	appendMode := false
	var files []string
	flagsDone := false

	for _, arg := range args {
		if flagsDone {
			files = append(files, arg)
			continue
		}

		if arg == "--" {
			flagsDone = true
			continue
		}

		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'a':
					appendMode = true
				default:
					fmt.Fprintf(os.Stderr, "sponge: invalid option -- '%c'\n", ch)
					fmt.Fprint(os.Stderr, usage)
					os.Exit(1)
				}
			}
			continue
		}

		files = append(files, arg)
	}

	outputFile := ""
	if len(files) > 0 {
		outputFile = files[0]
	}

	return appendMode, outputFile
}

// run reads all of stdin, then writes to the output file or stdout.
// Returns the exit code.
func run(appendMode bool, outputFile string) int {
	// R1.1: read all of stdin before opening the output file.
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: read error: %v\n", err)
		return 1
	}

	// R4.1: passthrough mode — write to stdout.
	if outputFile == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: write error: %v\n", err)
			return 1
		}
		return 0
	}

	// R2.3, R2.4: check existing file for permissions.
	var existingMode os.FileMode
	hasExisting := false
	if info, err := os.Lstat(outputFile); err == nil {
		if info.Mode().IsRegular() {
			hasExisting = true
			existingMode = info.Mode().Perm()
		}
	}

	// R3.1–R3.3: append mode — prepend original file content.
	if appendMode && hasExisting {
		original, err := os.ReadFile(outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			return 1
		}
		data = append(original, data...)
	}

	// R2.1: write to a temp file, then rename atomically.
	dir := filepath.Dir(outputFile)
	tmpFile, err := os.CreateTemp(dir, "sponge.")
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: cannot create temp file: %v\n", err)
		return 1
	}
	tmpPath := tmpFile.Name()

	// R5.4: clean up temp file on any failure path.
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(tmpPath) // best-effort cleanup
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close() // best-effort close
		fmt.Fprintf(os.Stderr, "sponge: write error: %v\n", err)
		return 1
	}
	if err := tmpFile.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: close error: %v\n", err)
		return 1
	}

	// R2.3: set permissions on temp file before rename.
	if hasExisting {
		if err := os.Chmod(tmpPath, existingMode); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: chmod: %v\n", err)
			return 1
		}
	} else {
		// Default mode 0666, masked by umask (os.CreateTemp uses 0600,
		// so set 0666 explicitly and let the kernel apply umask).
		if err := os.Chmod(tmpPath, 0o666); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: chmod: %v\n", err)
			return 1
		}
	}

	// R2.1: attempt atomic rename.
	if err := os.Rename(tmpPath, outputFile); err != nil {
		// R2.2: fallback to copy on cross-device rename.
		if err := copyFile(tmpPath, outputFile); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			return 1
		}
	}

	cleanup = false
	return 0
}

// copyFile copies src to dst as a fallback when rename fails. R2.2.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read temp file: %w", err)
	}

	// Get temp file mode to preserve it.
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat temp file: %w", err)
	}

	if err := os.WriteFile(dst, data, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}

	os.Remove(src) // best-effort cleanup of temp file
	return nil
}
