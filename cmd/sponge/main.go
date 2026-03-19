// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd007-sponge R1.1–R1.5, R2.1–R2.3: core stdin-to-file behavior
// with signal cleanup, atomic rename, rename fallback, and permission preservation.
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "sponge"

// initialBufSize is the starting buffer capacity per R1.2.
const initialBufSize = 8192

// spillThreshold is the in-memory limit before spilling to a temp file (R1.3).
const spillThreshold = 256 * 1024 * 1024

// cleanupMu protects cleanupPaths from concurrent access.
var cleanupMu sync.Mutex

// cleanupPaths tracks temp file paths for signal cleanup (R1.5).
var cleanupPaths []string

// soakedInput holds stdin content, either in memory or spilled to a temp file.
type soakedInput struct {
	buf     []byte   // in-memory data (nil when spilled)
	tmpFile *os.File // spill file (nil when in-memory)
	tmpPath string   // spill file path for cleanup
}

func main() {
	sys.InstallSIGPIPEHandler()
	installSignalCleanup()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// installSignalCleanup registers a handler that removes temp files on
// SIGINT, SIGTERM, or SIGHUP before the process exits. Implements R1.5.
func installSignalCleanup() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-ch
		cleanupMu.Lock()
		for _, p := range cleanupPaths {
			os.Remove(p) // best-effort cleanup
		}
		cleanupMu.Unlock()
		os.Exit(1)
	}()
}

// registerCleanup adds a path to the signal cleanup list (R1.5).
func registerCleanup(path string) {
	cleanupMu.Lock()
	cleanupPaths = append(cleanupPaths, path)
	cleanupMu.Unlock()
}

// unregisterCleanup removes a path from the signal cleanup list.
func unregisterCleanup(path string) {
	cleanupMu.Lock()
	for i, p := range cleanupPaths {
		if p == path {
			cleanupPaths = append(cleanupPaths[:i], cleanupPaths[i+1:]...)
			break
		}
	}
	cleanupMu.Unlock()
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
	if soak.tmpPath != "" {
		registerCleanup(soak.tmpPath)
	}
	defer func() {
		if soak.tmpPath != "" {
			unregisterCleanup(soak.tmpPath)
		}
		cleanupSoak(soak)
	}()
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

// writeFile writes soaked data to the named output file using atomic
// rename with copy fallback. Implements R2.1, R2.2, R2.3.
func writeFile(s *soakedInput, path string, appendMode bool, stderr io.Writer) int {
	mode := existingFileMode(path)
	tmpPath, err := writeOutputTemp(s, path, appendMode)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	registerCleanup(tmpPath)
	defer unregisterCleanup(tmpPath)
	if err := installOutput(tmpPath, path, mode); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
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

// writeOutputTemp creates a temp file with the final content and returns
// its path. Implements R1.4 (temp file location).
func writeOutputTemp(s *soakedInput, path string, appendMode bool) (string, error) {
	dir := os.Getenv("TMPDIR") // R1.4: platform context variable
	if dir == "" {
		dir = "/tmp"
	}
	f, err := os.CreateTemp(dir, "sponge.")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := f.Name()
	if err := writeOutputContent(f, s, path, appendMode); err != nil {
		f.Close()
		os.Remove(tmpPath) // best-effort cleanup
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath) // best-effort cleanup
		return "", fmt.Errorf("closing temp file: %w", err)
	}
	return tmpPath, nil
}

// writeOutputContent writes the final output to f: optional existing file
// content (append mode) followed by soaked stdin data.
func writeOutputContent(f *os.File, s *soakedInput, path string, appendMode bool) error {
	if appendMode {
		if data, err := os.ReadFile(path); err == nil {
			if _, err := f.Write(data); err != nil {
				return fmt.Errorf("writing prepend data: %w", err)
			}
		}
	}
	return writeSoakedTo(f, s)
}

// writeSoakedTo writes the soaked stdin data to w.
func writeSoakedTo(w io.Writer, s *soakedInput) error {
	if s.tmpFile == nil {
		_, err := w.Write(s.buf)
		return err
	}
	if _, err := s.tmpFile.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err := io.Copy(w, s.tmpFile)
	return err
}

// installOutput renames tmpPath to path atomically (R2.1), falling back
// to copy on failure (R2.2), and preserves permissions (R2.3).
func installOutput(tmpPath, path string, mode os.FileMode) error {
	if err := os.Rename(tmpPath, path); err == nil {
		os.Chmod(path, mode) // R2.3: best-effort permission preservation
		return nil
	}
	if err := fallbackCopy(tmpPath, path, mode); err != nil {
		return err
	}
	os.Remove(tmpPath) // best-effort cleanup of temp after copy
	return nil
}

// fallbackCopy copies src to dst when rename fails (R2.2), then applies
// the specified permission mode (R2.3).
func fallbackCopy(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening temp file for copy: %w", err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("opening output file: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copying to output file: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("closing output file: %w", err)
	}
	os.Chmod(dst, mode) // R2.3: best-effort permission preservation
	return nil
}
