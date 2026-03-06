// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cmd/sponge binary.
// Reads all of stdin into a buffer before writing any output, making the
// idiom "cmd file | sponge file" safe. Supports append mode (-a) and
// passthrough mode (no output filename).
//
// Implements: prd007-sponge R1-R5
// Architecture: docs/ARCHITECTURE.yaml § cmd/
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

const (
	// defaultFileMode is applied to new output files masked by umask (R2.3).
	defaultFileMode os.FileMode = 0o666

	// tempPattern is the temp file name pattern matching sponge.c mkstemp (R1.4).
	tempPattern = "sponge.*"
)

// cleanupMu guards cleanupFile against concurrent access from the signal
// handler goroutine and the main goroutine (R1.5).
var cleanupMu sync.Mutex //nolint:gochecknoglobals

// cleanupFile is the path to the temp file that must be removed on exit (R1.5).
var cleanupFile string //nolint:gochecknoglobals

// registerCleanup records a temp file path for signal-handler cleanup (R1.5).
func registerCleanup(path string) {
	cleanupMu.Lock()
	cleanupFile = path
	cleanupMu.Unlock()
}

// clearCleanup removes the registered cleanup path after a successful rename.
func clearCleanup() {
	cleanupMu.Lock()
	cleanupFile = ""
	cleanupMu.Unlock()
}

// doCleanup removes the registered temp file if one exists.
func doCleanup() {
	cleanupMu.Lock()
	path := cleanupFile
	cleanupFile = ""
	cleanupMu.Unlock()
	if path != "" {
		_ = os.Remove(path) // best-effort cleanup
	}
}

func main() {
	// R1.5: Install signal handler to clean up temp file on SIGINT, SIGTERM,
	// SIGHUP, or SIGPIPE before the rename completes.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)
	go func() {
		<-sigCh
		doCleanup()
		os.Exit(1)
	}()

	var appendMode bool
	flag.BoolVar(&appendMode, "a", false, "append stdin to file")
	flag.Parse()

	args := flag.Args()
	var outPath string
	if len(args) > 0 {
		outPath = args[0]
	}

	// R1.1, R1.2: Read all bytes from stdin before opening the output file.
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: read error: %v\n", err)
		os.Exit(1)
	}

	if outPath == "" {
		// R4.1, R4.3: Passthrough mode — write buffered stdin to stdout.
		if _, err := os.Stdout.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: write error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// R2: Write buffered content to the named output file.
	if err := writeToFile(outPath, data, appendMode); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		os.Exit(1)
	}
}

// writeToFile writes data to outPath via a temp file and atomic rename.
// When appendMode is true and outPath exists, the original file content is
// prepended to data (R3.1). File permissions are preserved via lstat (R2.3, R2.4).
func writeToFile(outPath string, data []byte, appendMode bool) error {
	// R2.3, R2.4: Read existing file permissions via lstat.
	mode := defaultFileMode
	info, err := os.Lstat(outPath)
	fileExists := err == nil

	if fileExists && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}

	// R3.1, R3.3: In append mode, prepend the original file content.
	if appendMode && fileExists {
		original, err := os.ReadFile(outPath)
		if err != nil {
			return fmt.Errorf("reading %s for append: %w", outPath, err)
		}
		combined := make([]byte, 0, len(original)+len(data))
		combined = append(combined, original...)
		combined = append(combined, data...)
		data = combined
	}

	// R2.1: Create temp file in the output directory for atomic rename.
	dir := filepath.Dir(outPath)
	tmpFile, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// R1.5: Register for signal cleanup.
	registerCleanup(tmpPath)

	// Write content to temp file.
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close() // best-effort cleanup
		doCleanup()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		doCleanup()
		return fmt.Errorf("closing temp file: %w", err)
	}

	// R2.3: Apply original file permissions to temp file.
	if err := os.Chmod(tmpPath, mode); err != nil {
		doCleanup()
		return fmt.Errorf("setting permissions: %w", err)
	}

	// R2.1: Attempt atomic rename.
	if err := os.Rename(tmpPath, outPath); err != nil {
		// R2.2: Rename failed (e.g., cross-device); fall back to copy.
		if cpErr := copyFile(tmpPath, outPath, mode); cpErr != nil {
			doCleanup()
			return fmt.Errorf("writing %s: %w", outPath, cpErr)
		}
		doCleanup()
		return nil
	}

	// Rename succeeded; clear cleanup registration.
	clearCleanup()
	return nil
}

// copyFile copies the contents of src to dst with the given permissions.
// Used as a fallback when os.Rename fails across device boundaries (R2.2).
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }() // best-effort cleanup

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if cerr := out.Close(); cerr != nil && err == nil {
		err = cerr
	}
	return err
}
