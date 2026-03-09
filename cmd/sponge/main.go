// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the sponge command: soak stdin and write to file.
// Implements prd007-sponge (R1-R5).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	appendMode, outFile := parseArgs(os.Args[1:])

	// R1.1: Read all of stdin before opening the output file.
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: read error: %v\n", err)
		os.Exit(1)
	}

	// R4.1: No output file, write buffered stdin to stdout.
	if outFile == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: write error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	writeToFile(outFile, data, appendMode)
}

// parseArgs parses command-line arguments and returns the append flag and
// output filename. Exits with code 1 on unknown flags.
func parseArgs(args []string) (appendMode bool, outFile string) {
	endOfFlags := false
	for _, arg := range args {
		if endOfFlags {
			outFile = arg
			break
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if arg == "-a" {
			appendMode = true
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			fmt.Fprintf(os.Stderr, "sponge: invalid option -- '%s'\n", arg[1:])
			os.Exit(1)
		}
		outFile = arg
		break
	}
	return appendMode, outFile
}

// writeToFile writes data to the output file using a temp-file-and-rename
// strategy for atomicity. In append mode, the original file content is
// prepended to the data.
func writeToFile(outFile string, data []byte, appendMode bool) {
	// R2.3, R2.4: Read existing file permissions via lstat.
	var mode os.FileMode = 0o666
	info, statErr := os.Lstat(outFile)
	if statErr == nil && info.Mode().IsRegular() {
		mode = info.Mode()
	}

	// R3.1, R3.2: In append mode, prepend original file content.
	if appendMode && statErr == nil && info.Mode().IsRegular() {
		original, err := os.ReadFile(outFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			os.Exit(1)
		}
		data = append(original, data...)
	}

	// R2.1: Write to temp file in the same directory, then rename.
	dir := filepath.Dir(outFile)
	if dir == "" {
		dir = "."
	}
	tmpFile, err := os.CreateTemp(dir, "sponge.")
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close() // best-effort close before cleanup
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "sponge: write error: %v\n", err)
		os.Exit(1)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "sponge: write error: %v\n", err)
		os.Exit(1)
	}

	// R2.3: Restore original file permissions on the temp file.
	if err := os.Chmod(tmpPath, mode); err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "sponge: chmod: %v\n", err)
		os.Exit(1)
	}

	// R2.1: Atomic rename.
	if err := os.Rename(tmpPath, outFile); err != nil {
		// R2.2: Cross-device fallback — copy content, then remove temp.
		content, readErr := os.ReadFile(tmpPath)
		os.Remove(tmpPath) // best-effort cleanup of temp file
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", readErr)
			os.Exit(1)
		}
		if writeErr := os.WriteFile(outFile, content, mode); writeErr != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", writeErr)
			os.Exit(1)
		}
	}
}
