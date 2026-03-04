// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the sponge utility for soaking stdin and writing to a file.
//
// Implements prd007-sponge (R1-R5).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

const (
	// R1.2: initial buffer size matching sponge.c BUFF_SIZE.
	initialBufferSize = 8192

	// R1.3: memory threshold for spilling stdin to a temp file.
	// Fixed threshold; the PRD permits "a fixed fraction of RLIMIT_DATA."
	spillThreshold = 256 * 1024 * 1024

	// permMask extracts permission and special bits from os.FileMode.
	permMask = os.ModeSetuid | os.ModeSetgid | os.ModeSticky | os.ModePerm
)

// tempFiles tracks temp file paths for cleanup on signal or exit.
var tempFiles []string

func main() {
	// R1.5: register cleanup handler for temp files on signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)
	go func() {
		<-sigCh
		cleanupAll()
		os.Exit(1)
	}()

	appendMode := flag.Bool("a", false, "append to the output file instead of truncating")
	flag.Parse()

	args := flag.Args()
	if len(args) > 1 {
		fmt.Fprintln(os.Stderr, "sponge: too many arguments")
		os.Exit(1)
	}

	var outPath string
	if len(args) == 1 {
		outPath = args[0]
	}

	// R1.1: read all of stdin before opening the output file.
	buf, spill, err := soak()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		closeFile(spill)
		cleanupAll()
		os.Exit(1)
	}

	var exitCode int
	if outPath == "" {
		// R4: passthrough mode -- write to stdout.
		if err := passthrough(buf, spill); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			exitCode = 1
		}
	} else {
		// R2, R3: write to named output file.
		if err := writeFile(outPath, buf, spill, *appendMode); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			exitCode = 1
		}
	}

	closeFile(spill)
	cleanupAll()
	os.Exit(exitCode)
}

// soak reads all of stdin into memory, spilling to a temp file if the buffer
// exceeds spillThreshold. R1.1, R1.2, R1.3, R1.4.
func soak() ([]byte, *os.File, error) {
	buf := make([]byte, 0, initialBufferSize)
	chunk := make([]byte, initialBufferSize)
	var spill *os.File

	for {
		n, err := os.Stdin.Read(chunk)
		if n > 0 {
			if spill != nil {
				if _, werr := spill.Write(chunk[:n]); werr != nil {
					return nil, spill, fmt.Errorf("writing to temp file: %w", werr)
				}
			} else {
				buf = append(buf, chunk[:n]...)
				// R1.3: spill buffer to temp file when threshold exceeded.
				if len(buf) > spillThreshold {
					sf, serr := makeTempFile()
					if serr != nil {
						return nil, nil, serr
					}
					spill = sf
					if _, werr := spill.Write(buf); werr != nil {
						return nil, spill, fmt.Errorf("writing to temp file: %w", werr)
					}
					buf = nil
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return buf, spill, nil
			}
			return buf, spill, fmt.Errorf("reading stdin: %w", err)
		}
	}
}

// passthrough writes the soaked stdin to stdout. R4.1, R4.2, R4.3.
func passthrough(buf []byte, spill *os.File) error {
	if spill != nil {
		// R4.2: seek spill file to start and copy to stdout.
		if _, err := spill.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("seeking temp file: %w", err)
		}
		if _, err := io.Copy(os.Stdout, spill); err != nil {
			return fmt.Errorf("writing to stdout: %w", err)
		}
		return nil
	}
	// R4.3: write in-memory buffer directly to stdout.
	if _, err := os.Stdout.Write(buf); err != nil {
		return fmt.Errorf("writing to stdout: %w", err)
	}
	return nil
}

// writeFile writes the soaked stdin to the named output file, optionally
// appending to existing content. R2, R3.
func writeFile(outPath string, buf []byte, spill *os.File, appendMode bool) error {
	// R2.4: lstat the output path.
	existingMode, isRegular := lstatMode(outPath)

	// Optimization: reuse spill file directly when no prepend is needed.
	if spill != nil && !(appendMode && isRegular) {
		return reuseSpill(spill, outPath, existingMode)
	}

	// Create a temp file for atomic output.
	tmp, err := makeTempFile()
	if err != nil {
		return err
	}

	// R3.1, R3.3: in append mode, prepend original file content.
	if appendMode && isRegular {
		if err := copyOriginal(outPath, tmp); err != nil {
			tmp.Close()
			return err
		}
	}

	// Write stdin content to the temp file.
	if err := writeStdin(tmp, buf, spill); err != nil {
		tmp.Close()
		return err
	}

	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	return atomicReplace(tmpPath, outPath, existingMode)
}

// lstatMode returns the file mode and whether the path is a regular file.
// R2.3, R2.4.
func lstatMode(path string) (os.FileMode, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		// R2.3: default mode for new files is 0666 masked by umask.
		return defaultMode(), false
	}
	if !info.Mode().IsRegular() {
		return defaultMode(), false
	}
	return info.Mode() & permMask, true
}

// defaultMode returns 0666 masked by the process umask. R2.3.
func defaultMode() os.FileMode {
	mask := syscall.Umask(0)
	syscall.Umask(mask)
	return os.FileMode(0o666 &^ mask)
}

// reuseSpill closes the spill file and renames it directly to the output path.
// Used in non-append mode when stdin was spilled to a temp file.
func reuseSpill(spill *os.File, outPath string, mode os.FileMode) error {
	spillPath := spill.Name()
	if err := spill.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	return atomicReplace(spillPath, outPath, mode)
}

// copyOriginal copies the existing file content into the temp file. R3.3.
func copyOriginal(origPath string, dst *os.File) error {
	src, err := os.Open(origPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", origPath, err)
	}
	defer src.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copying %s: %w", origPath, err)
	}
	return nil
}

// writeStdin writes the soaked stdin content to the temp file from either
// the in-memory buffer or the spill file.
func writeStdin(dst *os.File, buf []byte, spill *os.File) error {
	if spill != nil {
		if _, err := spill.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("seeking temp file: %w", err)
		}
		if _, err := io.Copy(dst, spill); err != nil {
			return fmt.Errorf("copying stdin: %w", err)
		}
		return nil
	}
	if len(buf) > 0 {
		if _, err := dst.Write(buf); err != nil {
			return fmt.Errorf("writing stdin: %w", err)
		}
	}
	return nil
}

// atomicReplace sets permissions on src, then renames it to dst. Falls back
// to a byte-for-byte copy when the rename fails (e.g., cross-device).
// R2.1, R2.2, R2.3.
func atomicReplace(src, dst string, mode os.FileMode) error {
	if err := os.Chmod(src, mode); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}
	// R2.5: untrack before rename so a signal handler won't delete the output.
	untrackTemp(src)
	if err := os.Rename(src, dst); err != nil {
		// R2.2: cross-device fallback.
		tempFiles = append(tempFiles, src)
		return fallbackCopy(src, dst, mode)
	}
	return nil
}

// fallbackCopy copies src to dst and removes src. R2.2.
func fallbackCopy(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening temp for copy: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("opening %s: %w", dst, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copying to %s: %w", dst, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", dst, err)
	}

	os.Remove(src) // best-effort cleanup of temp file
	untrackTemp(src)
	return nil
}

// makeTempFile creates a temp file in TMPDIR or /tmp and tracks it for
// cleanup. R1.4.
func makeTempFile() (*os.File, error) {
	dir := os.Getenv("TMPDIR")
	if dir == "" {
		dir = "/tmp"
	}
	f, err := os.CreateTemp(dir, "sponge.")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tempFiles = append(tempFiles, f.Name())
	return f, nil
}

// untrackTemp removes a path from the cleanup list.
func untrackTemp(path string) {
	for i, p := range tempFiles {
		if p == path {
			tempFiles = append(tempFiles[:i], tempFiles[i+1:]...)
			return
		}
	}
}

// cleanupAll removes all tracked temp files. R1.5, R5.4.
func cleanupAll() {
	for _, p := range tempFiles {
		os.Remove(p) // best-effort cleanup, error ignored
	}
	tempFiles = nil
}

// closeFile closes a file if non-nil. Used for best-effort cleanup.
func closeFile(f *os.File) {
	if f != nil {
		f.Close() // best-effort, error ignored on cleanup
	}
}
