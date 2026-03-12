// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd007-sponge R1.1–R1.5, R2.1–R2.5, R3.1–R3.2
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// Install SIGPIPE handler before any I/O per ARCHITECTURE shared_protocols.
	sys.InstallSIGPIPEHandler()

	// R3.1, R3.2: -a flag enables append mode.
	doAppend := flag.Bool("a", false, "append to output file")
	flag.Parse()
	args := flag.Args()

	// Accept zero or one positional FILE argument.
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "sponge: too many arguments\n")
		os.Exit(1)
	}

	// R1.1, R1.2: read all of stdin into memory before any file operation.
	buf, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: read error: %v\n", err)
		os.Exit(1)
	}

	// R4.1, R4.3: passthrough mode — write buffered stdin to stdout.
	if len(args) == 0 {
		if _, err := os.Stdout.Write(buf); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: write error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	outPath := args[0]
	if err := writeToFile(outPath, buf, *doAppend); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		os.Exit(1)
	}
}

// writeToFile writes buf atomically to outPath using a temp-file-then-rename
// strategy. It preserves the mode of an existing output file.
// When doAppend is true and the output file already exists as a regular file,
// prepends the existing file content before buf (R3.1–R3.2, R3.3).
//
// Implements: prd007-sponge R1.4, R1.5, R2.1–R2.5, R3.1–R3.2
func writeToFile(outPath string, buf []byte, doAppend bool) error {
	// R2.4: use sys.Lstat (not stat) to check whether the output path exists
	// and is a regular file (sponge.c:332-333). Symlinks are not followed;
	// a symlink at outPath does not count as an existing regular file.
	var existingMode os.FileMode
	fileExists := false
	if info, err := sys.Lstat(outPath); err == nil && info.Mode.IsRegular() {
		existingMode = info.Mode
		fileExists = true
	}

	// R1.4: create temp file in TMPDIR (or /tmp if TMPDIR is not set).
	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		tmpDir = "/tmp"
	}
	tmpFile, err := os.CreateTemp(tmpDir, "sponge.")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// R1.5: register cleanup so the temp file is removed if the process exits
	// or receives SIGINT, SIGTERM, SIGHUP, or SIGPIPE before rename completes.
	var cleanOnce sync.Once
	cleanupTmp := func() {
		cleanOnce.Do(func() {
			os.Remove(tmpPath) // best-effort cleanup; error ignored
		})
	}
	defer cleanupTmp()

	done := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)
	go func() {
		select {
		case <-sigCh:
			cleanupTmp()
			os.Exit(1)
		case <-done:
		}
	}()

	// R3.1, R3.2: in append mode, prepend the existing file content into the
	// temp file before writing the stdin buffer (sponge.c:352-357). Only
	// applies when the output file already exists and is a regular file;
	// when it does not exist, -a behaves identically to default mode.
	if doAppend && fileExists {
		origFile, err := os.Open(outPath)
		if err != nil {
			tmpFile.Close()
			return fmt.Errorf("opening %s for append: %w", outPath, err)
		}
		if _, err := io.Copy(tmpFile, origFile); err != nil {
			origFile.Close()
			tmpFile.Close()
			return fmt.Errorf("reading %s for append: %w", outPath, err)
		}
		origFile.Close()
	}

	// Write buffered stdin content to the temp file.
	if _, err := tmpFile.Write(buf); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// R2.3: determine target mode — preserve existing mode or default 0666
	// (which the OS will mask by the process umask).
	outMode := os.FileMode(0o666)
	if fileExists {
		outMode = existingMode
	}

	// R2.1: attempt atomic rename of temp file to output file.
	if err := os.Rename(tmpPath, outPath); err != nil {
		// R2.2: rename failed (e.g., cross-device link); fall back to copy
		// then remove the temp file.
		// R2.5: copy completes before the temp file (old source) is removed,
		// so the output file is never in a partially-written state without
		// the source available for recovery.
		if copyErr := copyAndReplace(tmpPath, outPath, outMode); copyErr != nil {
			return fmt.Errorf("writing %s: %w", outPath, copyErr)
		}
	} else {
		// R2.3: apply the desired mode to the renamed file; os.CreateTemp
		// creates with mode 0600 so an explicit chmod is required.
		if chmodErr := os.Chmod(outPath, outMode); chmodErr != nil {
			return fmt.Errorf("chmod %s: %w", outPath, chmodErr)
		}
	}

	// Unblock the signal goroutine so it exits cleanly.
	signal.Stop(sigCh)
	close(done)
	return nil
}

// copyAndReplace copies src to dst with the given mode, then removes src.
// Used as the cross-device fallback when os.Rename fails (R2.2).
func copyAndReplace(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("opening destination: %w", err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		return fmt.Errorf("copying data: %w", err)
	}
	if err := dstFile.Close(); err != nil {
		return fmt.Errorf("closing destination: %w", err)
	}

	os.Remove(src) // best-effort; error ignored after successful copy
	return nil
}
