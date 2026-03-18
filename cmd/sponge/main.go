// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd007-sponge R1.1–R1.4: core stdin-to-file behavior.
// R1.1: reads all stdin before opening output file.
// R1.2: dynamically grown buffer starting at 8192 bytes.
// R1.3: spills to temp file when memory threshold exceeded.
// R1.4: temp file in TMPDIR or /tmp with sponge.XXXXXX pattern.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "sponge"

// initialBufSize is the starting buffer capacity per R1.2.
const initialBufSize = 8192

// spillThreshold is the in-memory limit before spilling to a temp file (R1.3).
const spillThreshold = 256 * 1024 * 1024

// soakedInput holds stdin content, either in memory or spilled to a temp file.
type soakedInput struct {
	buf     []byte   // in-memory data (nil when spilled)
	tmpFile *os.File // spill file (nil when in-memory)
	tmpPath string   // spill file path for cleanup
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments, soaks stdin, and writes output. Returns exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	appendMode, outFile := parseArgs(args)
	soak, err := soakStdin(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		cleanupSoak(soak)
		return 1
	}
	defer cleanupSoak(soak)
	if outFile == "" {
		return writeStdout(soak, stdout, stderr)
	}
	return writeFile(soak, outFile, appendMode, stderr)
}

// parseArgs extracts the -a flag and optional output filename.
func parseArgs(args []string) (bool, string) {
	appendMode := false
	var outFile string
	for _, arg := range args {
		if arg == "-a" {
			appendMode = true
		} else {
			outFile = arg
		}
	}
	return appendMode, outFile
}

// soakStdin reads all stdin into memory, spilling to a temp file if
// the data exceeds spillThreshold. Implements R1.1, R1.2, R1.3.
func soakStdin(r io.Reader) (*soakedInput, error) {
	s := &soakedInput{buf: make([]byte, 0, initialBufSize)}
	chunk := make([]byte, initialBufSize)
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			if writeErr := appendToSoak(s, chunk[:n]); writeErr != nil {
				return s, writeErr
			}
		}
		if err == io.EOF {
			return s, nil
		}
		if err != nil {
			return s, err
		}
	}
}

// appendToSoak appends data to the soaked input, spilling to a temp file
// if the in-memory threshold is exceeded (R1.3).
func appendToSoak(s *soakedInput, data []byte) error {
	if s.tmpFile != nil {
		_, err := s.tmpFile.Write(data)
		return err
	}
	s.buf = append(s.buf, data...)
	if len(s.buf) <= spillThreshold {
		return nil
	}
	return spillToFile(s)
}

// spillToFile creates a temp file and writes the current buffer to it (R1.3, R1.4).
func spillToFile(s *soakedInput) error {
	dir := os.Getenv("TMPDIR") // R1.4: platform context variable
	if dir == "" {
		dir = "/tmp"
	}
	f, err := os.CreateTemp(dir, "sponge.")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	s.tmpFile = f
	s.tmpPath = f.Name()
	if _, err := f.Write(s.buf); err != nil {
		return fmt.Errorf("writing to temp file: %w", err)
	}
	s.buf = nil
	return nil
}

// cleanupSoak removes the temp file if one was created.
func cleanupSoak(s *soakedInput) {
	if s == nil || s.tmpFile == nil {
		return
	}
	s.tmpFile.Close()    // best-effort close
	os.Remove(s.tmpPath) // best-effort remove
}

// writeStdout writes soaked data to stdout (passthrough mode).
func writeStdout(s *soakedInput, stdout io.Writer, stderr io.Writer) int {
	if s.tmpFile != nil {
		return copySpillToWriter(s, stdout, stderr)
	}
	if _, err := stdout.Write(s.buf); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	return 0
}

// copySpillToWriter seeks the spill file to the start and copies to w.
func copySpillToWriter(s *soakedInput, w io.Writer, stderr io.Writer) int {
	if _, err := s.tmpFile.Seek(0, io.SeekStart); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if _, err := io.Copy(w, s.tmpFile); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	return 0
}

// writeFile writes soaked data to the named output file.
func writeFile(s *soakedInput, path string, appendMode bool, stderr io.Writer) int {
	mode := existingFileMode(path)
	content, err := buildContent(s, path, appendMode)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	os.Chmod(path, mode) // best-effort: preserve original mode
	return 0
}

// existingFileMode returns the permission mode of the file at path,
// or 0666 if the file does not exist. Uses Lstat per R2.4.
func existingFileMode(path string) os.FileMode {
	if info, err := os.Lstat(path); err == nil {
		return info.Mode().Perm()
	}
	return 0o666
}

// buildContent assembles the final output, prepending existing file content
// in append mode (-a).
func buildContent(s *soakedInput, path string, appendMode bool) ([]byte, error) {
	var prefix []byte
	if appendMode {
		if data, err := os.ReadFile(path); err == nil {
			prefix = data
		}
	}
	stdinData, err := readSoakedData(s)
	if err != nil {
		return nil, err
	}
	if prefix != nil {
		return append(prefix, stdinData...), nil
	}
	return stdinData, nil
}

// readSoakedData returns the full stdin content from the soaked input.
func readSoakedData(s *soakedInput) ([]byte, error) {
	if s.tmpFile == nil {
		return s.buf, nil
	}
	if _, err := s.tmpFile.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(s.tmpFile)
}
