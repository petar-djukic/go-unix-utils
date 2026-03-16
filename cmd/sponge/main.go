// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd007-sponge R1.1-R1.5, R2.1-R2.3, R3.1-R3.2.
// sponge reads all of stdin into a buffer before writing to the output file,
// enabling safe in-place pipeline rewrites. Supports -a append mode and
// passthrough to stdout when no filename is given. Installs SIGPIPE handler
// per ARCHITECTURE.yaml.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages.
const progName = "sponge"

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.5: Parse -a flag for append mode.
	var appendMode bool
	var outFile string
	for _, arg := range args {
		if arg == "-a" {
			appendMode = true
		} else {
			outFile = arg
		}
	}

	// R1.1: Read all of stdin before opening the output file.
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err)
		os.Exit(1)
	}

	// R1.2: No filename — write buffered stdin to stdout.
	if outFile == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			os.Exit(1)
		}
		return
	}

	// R3.1, R3.2: Append mode — append stdin to existing file content.
	if appendMode {
		if err := appendToFile(outFile, data); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, outFile, err)
			os.Exit(1)
		}
		return
	}

	// R1.3: Filename given — write to file via atomic rename.
	if err := writeToFile(outFile, data); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, outFile, err)
		os.Exit(1)
	}
}

// appendToFile appends data to the named file. If the file does not exist,
// it is created with mode 0666 (before umask). D2: opens with O_APPEND
// after all stdin has been read; does not use atomic rename.
func appendToFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o666)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close() // best-effort close
		return err
	}
	return f.Close()
}

// writeToFile writes data to the named file using a temp-file-then-rename
// strategy for atomicity. R2.1: temp file in same directory as target.
// R2.2: atomic rename with cross-device fallback. R2.3: preserves existing
// file permissions, or uses 0666 (before umask) for new files.
func writeToFile(path string, data []byte) error {
	// Determine target permissions: preserve existing, else default 0666.
	mode := os.FileMode(0o666)
	if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}

	// Create temp file in the same directory for same-device rename.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".sponge-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	// Clean up temp file on any failure path.
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmpName) // best-effort cleanup
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close() // best-effort close before cleanup
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}

	// Attempt atomic rename.
	if err := os.Rename(tmpName, path); err != nil {
		// Cross-device fallback: write content directly.
		if writeErr := os.WriteFile(path, data, mode); writeErr != nil {
			return writeErr
		}
		os.Remove(tmpName) // best-effort cleanup of temp
	}

	ok = true
	return nil
}
