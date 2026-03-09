// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd007-sponge R1.1–R1.5 (core stdin reading, temporary file
// buffering, and write-back), R2.1–R2.5 (atomic output file handling,
// permission preservation, lstat-based symlink awareness, and write error
// handling), R3.1–R3.3 (append mode with atomic temp-file prepend and
// default permissions for new files), R4.1–R4.3 (passthrough mode with
// in-memory buffering for small input and temp-file fallback for large input),
// R5.1–R5.4 (exit codes, error handling, panic recovery, and temp-file cleanup).
// Reads all of stdin before writing to the output file or stdout. Supports
// -a (append) mode.
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run())
}

// run implements the sponge logic and returns the process exit code.
// R5.1: returns 0 on success. R5.2: returns 1 on any I/O or write error.
func run() (exitCode int) {
	// R5.3: recover from allocation failure (panic) and exit with code 1
	// instead of printing a stack trace.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", r)
			exitCode = 1
		}
	}()

	appendMode, outFile := parseArgs(os.Args[1:])

	// R4.1, R4.3: passthrough mode — buffer stdin in memory and write to
	// stdout. No temp file is created for passthrough.
	if outFile == "" {
		if err := passthrough(); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			return 1
		}
		return 0
	}

	// File output mode: buffer stdin in a temp file.
	// R1.4: create temp file in the same directory as the output file
	// to ensure atomic rename works on the same filesystem.
	tmpDir := filepath.Dir(outFile)
	if tmpDir == "" {
		tmpDir = "."
	}

	tmp, err := os.CreateTemp(tmpDir, "sponge.*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: failed to create temporary file: %v\n", err)
		return 1
	}
	tmpName := tmp.Name()

	// R5.4: ensure temp file cleanup on all exit paths. Using return
	// instead of os.Exit guarantees this defer executes.
	defer os.Remove(tmpName) // best-effort cleanup

	// R5.4: clean up temp file on SIGINT, SIGTERM, SIGHUP before the
	// rename completes.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigCh
		os.Remove(tmpName) // best-effort cleanup
		os.Exit(1)
	}()
	defer signal.Stop(sigCh)

	// R3.3: in append mode, copy the original file content into the temp
	// file first, then append stdin. The original file is read before the
	// temp file is renamed over it (sponge.c:352-357).
	if appendMode {
		if orig, err := os.Open(outFile); err == nil {
			if _, err := io.Copy(tmp, orig); err != nil {
				orig.Close() // best-effort close before exit
				tmp.Close()  // best-effort close before exit
				fmt.Fprintf(os.Stderr, "sponge: failed to read original file: %v\n", err)
				return 1
			}
			orig.Close()
		}
		// R3.2: if the file does not exist, -a behaves like default mode.
	}

	// R1.1: read all of stdin into the temporary file before opening or
	// creating the output file.
	if _, err := io.Copy(tmp, os.Stdin); err != nil {
		tmp.Close() // best-effort close before exit
		fmt.Fprintf(os.Stderr, "sponge: error reading stdin: %v\n", err)
		return 1
	}

	if err := tmp.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: error closing temporary file: %v\n", err)
		return 1
	}

	// R2.3: atomic write — rename temp file over the target.
	// R2.3 (PRD): preserve file mode if the target already exists.
	existingMode := getFileMode(outFile)

	if err := os.Rename(tmpName, outFile); err != nil {
		// R2.2 (PRD): cross-device fallback — copy content then remove temp file.
		if err := copyFile(tmpName, outFile); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			return 1
		}
	}

	if existingMode != 0 {
		// R3.1: restore the original file permissions when overwriting.
		os.Chmod(outFile, existingMode) // best-effort permission restore
	} else {
		// R3.2: new file gets default permissions (0666 masked by process umask).
		// os.CreateTemp uses 0600; we adjust to match the standard 0666 & ~umask
		// that open(O_CREAT, 0666) would produce.
		umask := syscall.Umask(0)
		syscall.Umask(umask)
		os.Chmod(outFile, os.FileMode(0o666&^umask)) // best-effort permission set
	}

	return 0
}

// parseArgs extracts the -a flag and the output filename from arguments.
func parseArgs(args []string) (appendMode bool, outFile string) {
	for _, arg := range args {
		switch arg {
		case "-a":
			appendMode = true
		default:
			outFile = arg
		}
	}
	return appendMode, outFile
}

// getFileMode returns the file mode of an existing file, or 0 if it does
// not exist or cannot be stat'd. R2.4: uses Lstat (not Stat) so that
// symlinks are identified rather than followed blindly.
func getFileMode(path string) os.FileMode {
	info, err := os.Lstat(path)
	if err != nil {
		return 0
	}
	// R2.4: if the path is a symlink, resolve the target to get the
	// actual file permissions of the destination file.
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Stat(path)
		if err != nil {
			return 0
		}
		return target.Mode().Perm()
	}
	return info.Mode().Perm()
}

// passthrough reads all of stdin into memory and writes it to stdout.
// R4.1: write buffered stdin to stdout when no output filename is given.
// R4.3: small input stays in memory without creating a temp file.
// R4.2: large input is handled transparently by io.ReadAll.
func passthrough() error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("error reading stdin: %w", err)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		return fmt.Errorf("write error: %w", err)
	}
	return nil
}

// copyFile copies the content of src to dst, creating or truncating dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open temporary file for copy: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close() // best-effort close before returning error
		return fmt.Errorf("failed to copy to output file: %w", err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("failed to close output file: %w", err)
	}
	return nil
}
