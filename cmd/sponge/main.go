// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the sponge utility.
// Implements srd007-sponge R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// initialBufSize is the starting buffer capacity.
// R1.2: matches sponge.c:BUFF_SIZE (8192).
const initialBufSize = 8192

// spillThreshold is the memory limit before spilling to a temp file.
// R1.3: spill when in-memory buffer exceeds this size.
const spillThreshold = 256 * 1024 * 1024

func main() {
	// R1.4: install SIGPIPE handler per srd002-sys.
	sys.InstallSIGPIPEHandler()

	outFile := ""
	if len(os.Args) > 1 {
		outFile = os.Args[1]
	}

	data, tmpPath, err := readAllStdin()
	if tmpPath != "" {
		defer os.Remove(tmpPath) // best-effort cleanup
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		os.Exit(1)
	}

	if err := writeOutput(outFile, data, tmpPath); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		os.Exit(1)
	}
}

// readAllStdin reads all of stdin into memory or a temp file.
// R1.1: all stdin is consumed before this function returns.
// R1.2: buffer starts at initialBufSize and grows dynamically.
// R1.3: spills to temp file when buffer exceeds spillThreshold.
func readAllStdin() ([]byte, string, error) {
	buf := bytes.NewBuffer(make([]byte, 0, initialBufSize))
	n, err := io.CopyN(buf, os.Stdin, int64(spillThreshold)+1)
	if err != nil && err != io.EOF {
		return nil, "", fmt.Errorf("reading stdin: %w", err)
	}
	if n <= int64(spillThreshold) {
		return buf.Bytes(), "", nil
	}
	return spillAndContinue(buf, os.Stdin)
}

// spillAndContinue writes the in-memory buffer to a temp file and
// continues reading remaining stdin into it.
// R1.3: transparent spill to temp file.
// R1.4: temp file in TMPDIR or /tmp with sponge pattern.
func spillAndContinue(buf *bytes.Buffer, remaining io.Reader) ([]byte, string, error) {
	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		tmpDir = "/tmp"
	}
	f, err := os.CreateTemp(tmpDir, "sponge.")
	if err != nil {
		return nil, "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := f.Name()
	if _, err := buf.WriteTo(f); err != nil {
		f.Close()          // best-effort close
		os.Remove(tmpPath) // best-effort cleanup
		return nil, "", fmt.Errorf("writing temp file: %w", err)
	}
	if _, err := io.Copy(f, remaining); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return nil, "", fmt.Errorf("reading stdin: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return nil, "", fmt.Errorf("closing temp file: %w", err)
	}
	return nil, tmpPath, nil
}

// writeOutput dispatches to stdout or file output.
func writeOutput(outFile string, data []byte, tmpPath string) error {
	if outFile == "" {
		return writeToStdout(data, tmpPath)
	}
	return writeToFile(outFile, data, tmpPath)
}

// writeToStdout writes buffered content to stdout.
// R1.3 (passthrough): writes in-memory buffer or temp file to stdout.
func writeToStdout(data []byte, tmpPath string) error {
	if tmpPath != "" {
		return copyFileToWriter(tmpPath, os.Stdout)
	}
	_, err := os.Stdout.Write(data)
	return err
}

// writeToFile writes buffered content to the named output file.
// R1.1: file is not opened until stdin is fully consumed.
func writeToFile(outFile string, data []byte, tmpPath string) error {
	f, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("opening %s: %w", outFile, err)
	}
	defer f.Close()
	if tmpPath != "" {
		return copyFileToWriter(tmpPath, f)
	}
	_, err = f.Write(data)
	return err
}

// copyFileToWriter copies the content of a file to a writer.
func copyFileToWriter(path string, w io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening temp file: %w", err)
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}
