// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd007-sponge R1.1–R1.5, R2.1–R2.5, R3.1–R3.3, R4.1–R4.3, R5.1–R5.4:
// core stdin-to-file behavior with signal cleanup, atomic rename, rename
// fallback, permission preservation, lstat-based output checks, append mode
// error handling, and passthrough mode.
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
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

// outputInfo holds lstat results for the output path (R2.4).
type outputInfo struct {
	mode      os.FileMode
	exists    bool
	isRegular bool
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
// R5.1: returns 0 on success. R5.2: returns 1 on all error paths.
// R5.3: recovers from allocation panics and exits 1 instead of crashing.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) (exitCode int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "%s: %v\n", progName, r)
			exitCode = 1
		}
	}()
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
// R4.1: writes buffered stdin to stdout when no output filename given.
// R4.2: seeks spill file to start and copies to stdout for large input.
// R4.3: writes in-memory buffer directly for small input.
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

// checkOutputPath uses lstat to determine output file state (R2.4).
func checkOutputPath(path string) outputInfo {
	info, err := os.Lstat(path)
	if err != nil {
		return outputInfo{mode: 0o666, exists: false, isRegular: false}
	}
	return outputInfo{
		mode:      info.Mode().Perm(),
		exists:    true,
		isRegular: info.Mode().IsRegular(),
	}
}

// writeFile writes soaked data to the named output file using atomic
// rename with copy fallback. Implements R2.1–R2.5, R3.1–R3.2.
func writeFile(s *soakedInput, path string, appendMode bool, stderr io.Writer) int {
	outInfo := checkOutputPath(path)
	tmpPath, err := writeOutputTemp(s, path, appendMode, outInfo)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	registerCleanup(tmpPath)
	defer func() {
		unregisterCleanup(tmpPath)
		os.Remove(tmpPath) // R5.4: best-effort cleanup on all exit paths
	}()
	if err := installOutput(tmpPath, path, outInfo); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	return 0
}

// writeOutputTemp creates a temp file with the final content and returns
// its path. Implements R1.4 (temp file location).
func writeOutputTemp(s *soakedInput, path string, appendMode bool, info outputInfo) (string, error) {
	dir := os.Getenv("TMPDIR") // R1.4: platform context variable
	if dir == "" {
		dir = "/tmp"
	}
	f, err := os.CreateTemp(dir, "sponge.")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := f.Name()
	if err := writeOutputContent(f, s, path, appendMode, info); err != nil {
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
// content (append mode, R3.1) followed by soaked stdin data.
// R3.2: append only prepends when the file exists and is a regular file.
// R3.3: reads original file before writing stdin, propagates read errors.
func writeOutputContent(f *os.File, s *soakedInput, path string, appendMode bool, info outputInfo) error {
	if appendMode && info.exists && info.isRegular {
		if err := prependOriginalFile(f, path); err != nil {
			return err
		}
	}
	return writeSoakedTo(f, s)
}

// prependOriginalFile reads the original file and writes its content to f.
// Implements R3.3: original file must be read before the temp file is renamed.
func prependOriginalFile(f *os.File, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading original file for append: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing prepend data: %w", err)
	}
	return nil
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

// installOutput installs the temp file as the output file.
// R2.4: only attempt rename for regular files or non-existent paths.
// R2.5: copy path uses temp-in-target-dir + rename for atomicity.
func installOutput(tmpPath, path string, info outputInfo) error {
	if !info.exists || info.isRegular {
		if err := os.Rename(tmpPath, path); err == nil {
			os.Chmod(path, info.mode) // R2.3: best-effort permission preservation
			return nil
		}
		// R2.2, R2.5: rename failed (cross-device), atomic copy fallback
		return atomicCopyAndCleanup(tmpPath, path, info.mode)
	}
	// R2.4: non-regular file (e.g., symlink), direct copy follows the path
	return directCopyAndCleanup(tmpPath, path, info.mode)
}

// atomicCopyAndCleanup copies src to a temp file in dst's directory, then
// renames atomically to dst. Removes src on success. Implements R2.5.
func atomicCopyAndCleanup(src, dst string, mode os.FileMode) error {
	if err := atomicCopy(src, dst, mode); err != nil {
		return err
	}
	os.Remove(src) // best-effort cleanup of original temp
	return nil
}

// atomicCopy copies src content to a temp file in dst's directory, then
// renames it to dst for atomic replacement. Implements R2.5.
func atomicCopy(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening temp file for copy: %w", err)
	}
	defer in.Close()
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".sponge-copy.")
	if err != nil {
		return fmt.Errorf("creating copy temp file: %w", err)
	}
	tmpName := tmp.Name()
	if err := copyCloseAndRename(in, tmp, tmpName, dst, mode); err != nil {
		os.Remove(tmpName) // best-effort cleanup
		return err
	}
	return nil
}

// copyCloseAndRename copies data from in to tmp, closes tmp, sets mode,
// and renames tmpName to dst.
func copyCloseAndRename(in io.Reader, tmp *os.File, tmpName, dst string, mode os.FileMode) error {
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return fmt.Errorf("copying to output file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing copy temp: %w", err)
	}
	os.Chmod(tmpName, mode) // R2.3: best-effort permission preservation
	if err := os.Rename(tmpName, dst); err != nil {
		return fmt.Errorf("installing output file: %w", err)
	}
	return nil
}

// directCopyAndCleanup copies src to dst directly and removes src.
// Used for non-regular output paths (e.g., symlinks) per R2.4.
func directCopyAndCleanup(src, dst string, mode os.FileMode) error {
	if err := directCopy(src, dst, mode); err != nil {
		return err
	}
	os.Remove(src) // best-effort cleanup of original temp
	return nil
}

// directCopy copies src to dst by opening dst directly. When dst is a
// symlink, the OS follows it to the target. Used for R2.4 non-regular paths.
func directCopy(src, dst string, mode os.FileMode) error {
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
