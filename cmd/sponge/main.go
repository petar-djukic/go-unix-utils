// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd007-sponge R1.1–R1.5 (core stdin reading, temporary file
// buffering, and write-back), R2.1–R2.3 (atomic output file handling and
// permission preservation). Reads all of stdin into a temporary file before
// writing to the output file or stdout. Supports -a (append) mode.
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

	appendMode, outFile := parseArgs(os.Args[1:])

	// R1.4: create temp file in the same directory as the output file
	// to ensure atomic rename works on the same filesystem. When no
	// filename is given, use the system temp directory.
	tmpDir := os.TempDir()
	if outFile != "" {
		tmpDir = filepath.Dir(outFile)
		if tmpDir == "" {
			tmpDir = "."
		}
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
		// R4.1: no filename argument — write buffered stdin to stdout.
		if err := writeToStdout(tmpName); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if appendMode {
		// R2.1/R3.1: append buffered stdin to the target file.
		// R2.2/R3.2: if the file does not exist, create it.
		if err := appendToFile(tmpName, outFile); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// R2.3: atomic write — rename temp file over the target.
	// R2.3 (PRD): preserve file mode if the target already exists.
	existingMode := getFileMode(outFile)

	if err := os.Rename(tmpName, outFile); err != nil {
		// R2.2 (PRD): cross-device fallback — copy content then remove temp file.
		if err := copyFile(tmpName, outFile); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			os.Exit(1)
		}
	}

	// Restore the original file permissions if the target existed.
	if existingMode != 0 {
		os.Chmod(outFile, existingMode) // best-effort permission restore
	}
}

// parseArgs extracts the -a flag and the output filename from arguments.
func parseArgs(args []string) (appendMode bool, outFile string) {
	for _, arg := range args {
		switch arg {
		case "-a":
			appendMode = true
		default:
			outFile = arg
		}
	}
	return appendMode, outFile
}

// getFileMode returns the file mode of an existing file, or 0 if it does
// not exist or cannot be stat'd.
func getFileMode(path string) os.FileMode {
	info, err := os.Lstat(path)
	if err != nil {
		return 0
	}
	return info.Mode().Perm()
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

// appendToFile appends the content of the temp file to the target file.
// R2.1: when -a is specified, append buffered stdin to the target.
// R2.2: if the target does not exist, create it.
func appendToFile(tmpName, dst string) error {
	in, err := os.Open(tmpName)
	if err != nil {
		return fmt.Errorf("failed to open temporary file: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close() // best-effort close before returning error
		return fmt.Errorf("failed to append to output file: %w", err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("failed to close output file: %w", err)
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
