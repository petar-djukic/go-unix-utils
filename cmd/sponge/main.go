// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the sponge utility for soaking stdin before writing.
// Implements prd007-sponge (R1, R2, R3, R4, R5).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R5.4: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	appendMode, outFile := parseArgs(os.Args[1:])

	// R1.1: Read all stdin before opening the output file.
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: read error: %s\n", err)
		os.Exit(1)
	}

	// R4.1: No output filename -> write to stdout.
	if outFile == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			os.Exit(1) // SIGPIPE handled by handler
		}
		return
	}

	// R2.3, R2.4: Read existing file mode via lstat.
	var mode os.FileMode = 0o666
	if info, err := os.Lstat(outFile); err == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}

	// R3.1: In append mode, prepend existing content.
	if appendMode {
		if existing, err := os.ReadFile(outFile); err == nil {
			data = append(existing, data...)
		}
		// R3.2: If file doesn't exist, -a behaves as default.
	}

	// R2.1: Write via temp file + atomic rename.
	if err := writeAtomic(outFile, data, mode); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %s\n", err)
		os.Exit(1)
	}
}

// parseArgs parses command-line arguments into append flag and output filename.
func parseArgs(args []string) (appendMode bool, outFile string) {
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
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
			i++
			continue
		}
		break
	}

	if i < len(args) {
		outFile = args[i]
	}

	return appendMode, outFile
}

// writeAtomic writes data to outFile via a temp file and atomic rename.
// R2.1: Atomic rename. R2.2: Cross-device copy fallback. R5.4: Temp file cleanup.
func writeAtomic(outFile string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(outFile)
	if dir == "" {
		dir = "."
	}

	tmpFile, err := os.CreateTemp(dir, "sponge.")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName) // best-effort cleanup; no-op after successful rename

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close() // best-effort close
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}

	// R2.1: Try atomic rename.
	if err := os.Rename(tmpName, outFile); err != nil {
		// R2.2: Cross-device fallback.
		if writeErr := os.WriteFile(outFile, data, mode); writeErr != nil {
			return fmt.Errorf("writing output: %w", writeErr)
		}
	}

	return nil
}
