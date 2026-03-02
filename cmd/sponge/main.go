// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the sponge command, which reads all of standard input
// before writing to a named file, enabling safe pipelines like "cmd file | sponge file".
//
// Implements: prd007-sponge R1, R2, R3, R4, R5
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

// initialBufSize is the starting buffer capacity, matching sponge.c BUFF_SIZE.
//
// Implements: prd007-sponge R1.2
const initialBufSize = 8192

// spillThreshold is the in-memory buffer size beyond which stdin content is
// spilled to a temporary file. This approximates the reference implementation's
// physmem_available() check using a fixed fraction of typical available memory.
//
// Implements: prd007-sponge R1.3
const spillThreshold = 256 * 1024 * 1024

func main() {
	os.Exit(run(os.Args[1:]))
}

// run parses flags and executes the sponge operation. Returns 0 on success,
// 1 on error.
//
// Implements: prd007-sponge R1, R2, R3, R4, R5
func run(args []string) int {
	fs := flag.NewFlagSet("sponge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	flagAppend := fs.Bool("a", false, "append to the output file")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	files := fs.Args()
	var outFile string
	if len(files) > 0 {
		outFile = files[0]
	}

	// Read all stdin before touching the output file (R1.1).
	buf, spillPath, err := readAllStdin()
	if err != nil {
		printError(err)
		cleanupSpill(spillPath)
		return 1
	}

	// Register signal-based cleanup for spill temp file (R1.5).
	if spillPath != "" {
		installCleanupHandler(spillPath)
		defer cleanupSpill(spillPath)
	}

	// Passthrough mode: no output filename (R4.1).
	if outFile == "" {
		if err := writeToStdout(buf, spillPath); err != nil {
			printError(err)
			return 1
		}
		return 0
	}

	// Write to named output file (R2, R3).
	if err := writeToFile(outFile, buf, spillPath, *flagAppend); err != nil {
		printError(err)
		return 1
	}

	return 0
}

// readAllStdin reads all of stdin into memory or a temporary file.
// Returns (data, "", nil) when stdin fits in memory, or (nil, tempPath, nil)
// when stdin was spilled to a temp file.
//
// Implements: prd007-sponge R1.1, R1.2, R1.3, R1.4
func readAllStdin() ([]byte, string, error) {
	buf := make([]byte, 0, initialBufSize)
	chunk := make([]byte, initialBufSize)

	for {
		n, err := os.Stdin.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
		}
		if err == io.EOF {
			return buf, "", nil
		}
		if err != nil {
			return nil, "", fmt.Errorf("read error: %v", err)
		}

		// Spill to temp file when threshold exceeded (R1.3).
		if len(buf) >= spillThreshold {
			spillPath, spillErr := spillToTempFile(buf)
			if spillErr != nil {
				return nil, "", spillErr
			}
			// Continue reading remaining stdin directly to temp file.
			if appendErr := appendStdinToFile(spillPath); appendErr != nil {
				return nil, spillPath, appendErr
			}
			return nil, spillPath, nil
		}
	}
}

// spillToTempFile writes the current in-memory buffer to a temp file in
// TMPDIR (or /tmp) and returns the temp file path.
//
// Implements: prd007-sponge R1.3, R1.4
func spillToTempFile(buf []byte) (string, error) {
	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		tmpDir = "/tmp"
	}

	f, err := os.CreateTemp(tmpDir, "sponge.")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %v", err)
	}
	name := f.Name()

	if _, err := f.Write(buf); err != nil {
		f.Close()
		return name, fmt.Errorf("writing temp file: %v", err)
	}

	if err := f.Close(); err != nil {
		return name, fmt.Errorf("closing temp file: %v", err)
	}

	return name, nil
}

// appendStdinToFile reads remaining stdin and appends it to the given file.
func appendStdinToFile(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("opening temp file for append: %v", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, os.Stdin); err != nil {
		return fmt.Errorf("writing to temp file: %v", err)
	}

	return nil
}

// writeToStdout writes the buffered stdin content to stdout.
//
// Implements: prd007-sponge R4.1, R4.2, R4.3
func writeToStdout(buf []byte, spillPath string) error {
	if buf != nil {
		// In-memory buffer (R4.3).
		if _, err := os.Stdout.Write(buf); err != nil {
			return fmt.Errorf("write error: %v", err)
		}
		return nil
	}

	// Spilled to temp file (R4.2).
	f, err := os.Open(spillPath)
	if err != nil {
		return fmt.Errorf("opening temp file: %v", err)
	}
	defer f.Close()

	if _, err := io.Copy(os.Stdout, f); err != nil {
		return fmt.Errorf("write error: %v", err)
	}

	return nil
}

// writeToFile writes buffered stdin content to the named output file,
// using atomic rename with cross-device fallback and permission preservation.
//
// Implements: prd007-sponge R2, R3
func writeToFile(outFile string, buf []byte, spillPath string, appendMode bool) error {
	// Check existing file permissions via lstat (R2.3, R2.4).
	var mode os.FileMode = 0666
	info, err := os.Lstat(outFile)
	if err == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}

	// Create temp file in same directory as target for atomic rename (R2.1).
	dir := filepath.Dir(outFile)
	if dir == "" {
		dir = "."
	}
	tmpOut, err := os.CreateTemp(dir, ".sponge-tmp-")
	if err != nil {
		return fmt.Errorf("%s: %v", outFile, err)
	}
	tmpOutPath := tmpOut.Name()
	success := false
	defer func() {
		if !success {
			os.Remove(tmpOutPath) // best-effort cleanup on failure
		}
	}()

	// In append mode, prepend existing file content (R3.1, R3.3).
	if appendMode {
		if err := prependExistingContent(tmpOut, outFile); err != nil {
			tmpOut.Close()
			return err
		}
	}

	// Write stdin content to temp file.
	if err := writeStdinContent(tmpOut, buf, spillPath); err != nil {
		tmpOut.Close()
		return err
	}

	if err := tmpOut.Close(); err != nil {
		return fmt.Errorf("closing temp file: %v", err)
	}

	// Set permissions before rename (R2.3).
	if err := os.Chmod(tmpOutPath, mode); err != nil {
		return fmt.Errorf("setting permissions: %v", err)
	}

	// Atomic rename (R2.1).
	if err := os.Rename(tmpOutPath, outFile); err != nil {
		// Cross-device fallback (R2.2).
		if copyErr := copyFile(tmpOutPath, outFile, mode); copyErr != nil {
			return copyErr
		}
		os.Remove(tmpOutPath) // clean up temp after successful copy
	}

	success = true
	return nil
}

// prependExistingContent copies the existing content of outFile into dst.
// If outFile does not exist, this is a no-op (R3.2).
//
// Implements: prd007-sponge R3.1, R3.3
func prependExistingContent(dst *os.File, outFile string) error {
	src, err := os.Open(outFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // R3.2: non-existent file, behave as default mode.
		}
		return fmt.Errorf("%s: %v", outFile, err)
	}
	defer src.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("reading %s: %v", outFile, err)
	}

	return nil
}

// writeStdinContent writes the buffered stdin content to dst, reading from
// either the in-memory buffer or the spill temp file.
func writeStdinContent(dst *os.File, buf []byte, spillPath string) error {
	if buf != nil {
		if _, err := dst.Write(buf); err != nil {
			return fmt.Errorf("writing output: %v", err)
		}
		return nil
	}

	if spillPath == "" {
		return nil
	}

	src, err := os.Open(spillPath)
	if err != nil {
		return fmt.Errorf("opening temp file: %v", err)
	}
	defer src.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("writing output: %v", err)
	}

	return nil
}

// copyFile copies src to dst with the given file mode. Used as fallback
// when atomic rename fails due to cross-device move.
//
// Implements: prd007-sponge R2.2
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %v", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("%s: %v", dst, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("writing %s: %v", dst, err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("closing %s: %v", dst, err)
	}

	return nil
}

// installCleanupHandler registers signal handlers that delete the temp file
// before exiting. Handles SIGINT, SIGTERM, SIGHUP, and SIGPIPE.
//
// Implements: prd007-sponge R1.5
func installCleanupHandler(tmpPath string) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)
	go func() {
		<-c
		os.Remove(tmpPath) // best-effort cleanup
		os.Exit(1)
	}()
}

// cleanupSpill removes the spill temp file if it exists.
//
// Implements: prd007-sponge R5.4
func cleanupSpill(path string) {
	if path != "" {
		os.Remove(path) // best-effort; file may already be renamed or deleted
	}
}

// printError writes an error message to stderr in the format "sponge: <description>".
//
// Implements: prd007-sponge R5.2
func printError(err error) {
	fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
}
