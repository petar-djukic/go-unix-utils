// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sponge implements moreutils sponge: soak stdin before writing output.
//
// Implements prd007-sponge R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// initialBufSize matches sponge.c BUFF_SIZE (R1.2).
const initialBufSize = 8192

// tempPrefix is the mkstemp-style prefix for temp files (R1.4).
const tempPrefix = "sponge."

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run implements the sponge logic: read all stdin, then write output.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	outFile := ""
	if len(args) > 0 {
		outFile = args[len(args)-1]
	}

	buf, tmpPath, err := soakStdin(stdin)
	if tmpPath != "" {
		defer os.Remove(tmpPath) // best-effort cleanup (R1.4)
	}
	if err != nil {
		fmt.Fprintf(stderr, "sponge: %v\n", err)
		return 1
	}

	if outFile == "" {
		return writeToStdout(buf, tmpPath, stdout, stderr)
	}
	return writeToFile(buf, tmpPath, outFile, stderr)
}

// soakStdin reads all of stdin into memory or spills to a temp file (R1.1, R1.2, R1.3).
// Returns the in-memory buffer (nil if spilled), the temp file path (empty if in-memory), and any error.
func soakStdin(stdin io.Reader) ([]byte, string, error) {
	buf := make([]byte, 0, initialBufSize)
	threshold := memoryThreshold()

	for {
		if cap(buf)-len(buf) == 0 {
			newCap := max(cap(buf)*2, initialBufSize)
			grown := make([]byte, len(buf), newCap)
			copy(grown, buf)
			buf = grown
		}

		n, err := stdin.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]

		if uint64(len(buf)) > threshold {
			return spillToTempFile(buf, stdin)
		}

		if err == io.EOF {
			return buf, "", nil
		}
		if err != nil {
			return nil, "", fmt.Errorf("read stdin: %w", err)
		}
	}
}

// memoryThreshold returns the byte threshold at which stdin spills to a temp file (R1.3).
func memoryThreshold() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	avail := m.Sys - m.HeapInuse
	if avail < initialBufSize*4 {
		avail = 64 * 1024 * 1024 // 64 MB fallback
	}
	return avail / 2
}

// spillToTempFile writes the current buffer and remaining stdin to a temp file (R1.3, R1.4).
func spillToTempFile(buf []byte, remaining io.Reader) ([]byte, string, error) {
	tmpDir := os.Getenv("TMPDIR") // platform context: temp directory
	if tmpDir == "" {
		tmpDir = "/tmp"
	}

	f, err := os.CreateTemp(tmpDir, tempPrefix)
	if err != nil {
		return nil, "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := f.Name()

	if _, err := f.Write(buf); err != nil {
		f.Close() // best-effort close before cleanup
		return nil, tmpPath, fmt.Errorf("write to temp file: %w", err)
	}

	if _, err := io.Copy(f, remaining); err != nil {
		f.Close() // best-effort close before cleanup
		return nil, tmpPath, fmt.Errorf("copy stdin to temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		return nil, tmpPath, fmt.Errorf("close temp file: %w", err)
	}

	return nil, tmpPath, nil
}

// writeToStdout writes the buffered content to stdout (R4.1, R4.2, R4.3).
func writeToStdout(buf []byte, tmpPath string, stdout io.Writer, stderr io.Writer) int {
	if tmpPath != "" {
		if err := copyFileToWriter(tmpPath, stdout); err != nil {
			fmt.Fprintf(stderr, "sponge: %v\n", err)
			return 1
		}
		return 0
	}
	if _, err := stdout.Write(buf); err != nil {
		fmt.Fprintf(stderr, "sponge: %v\n", err)
		return 1
	}
	return 0
}

// writeToFile writes the buffered content to the named output file (R1.1, R2.1).
func writeToFile(buf []byte, tmpPath string, outFile string, stderr io.Writer) int {
	if tmpPath != "" {
		if err := renameOrCopy(tmpPath, outFile); err != nil {
			fmt.Fprintf(stderr, "sponge: %v\n", err)
			return 1
		}
		return 0
	}
	if err := os.WriteFile(outFile, buf, 0o666); err != nil {
		fmt.Fprintf(stderr, "sponge: %v\n", err)
		return 1
	}
	return 0
}

// renameOrCopy attempts an atomic rename; falls back to copy on failure (R2.1, R2.2).
func renameOrCopy(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	return copyFile(src, dst)
}

// copyFile copies src to dst by reading and writing content.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create output file %s: %w", filepath.Base(dst), err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy to output file: %w", err)
	}
	return out.Close()
}

// copyFileToWriter copies the content of the file at path to w.
func copyFileToWriter(path string, w io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("write to stdout: %w", err)
	}
	return nil
}
