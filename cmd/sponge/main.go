// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the sponge utility.
// Implements srd007-sponge R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// initialBufSize is the starting buffer capacity.
// R1.2: matches sponge.c:BUFF_SIZE (8192).
const initialBufSize = 8192

// spillThreshold is the memory limit before spilling to a temp file.
// R1.3: spill when in-memory buffer exceeds this size.
const spillThreshold = 256 * 1024 * 1024

// tempMu protects tempFilePath for concurrent signal handler access.
// R1.5: mutex ensures safe access from signal goroutine.
var tempMu sync.Mutex

// tempFilePath tracks the current temp file for signal cleanup.
var tempFilePath string

func main() {
	sys.InstallSIGPIPEHandler()
	installCleanupHandler()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		cleanupTempFile()
		os.Exit(1)
	}
}

// run executes the sponge logic, returning any error.
func run() error {
	_, outFile := parseArgs()

	data, tmpPath, err := readAllStdin()
	setTempFile(tmpPath)
	defer cleanupTempFile()

	if err != nil {
		return err
	}
	return writeOutput(outFile, data, tmpPath)
}

// parseArgs parses command-line arguments for flags and output filename.
func parseArgs() (bool, string) {
	appendMode := false
	args := os.Args[1:]
	for len(args) > 0 && len(args[0]) > 0 && args[0][0] == '-' {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		if args[0] == "-a" {
			appendMode = true
			args = args[1:]
			continue
		}
		break
	}
	outFile := ""
	if len(args) > 0 {
		outFile = args[0]
	}
	return appendMode, outFile
}

// installCleanupHandler registers signal handlers for temp file cleanup.
// R1.5: cleans up on SIGINT, SIGTERM, SIGHUP.
func installCleanupHandler() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-ch
		cleanupTempFile()
		os.Exit(1)
	}()
}

// setTempFile records the temp file path for signal cleanup.
func setTempFile(path string) {
	tempMu.Lock()
	tempFilePath = path
	tempMu.Unlock()
}

// clearTempFile clears the tracked temp file path.
func clearTempFile() {
	tempMu.Lock()
	tempFilePath = ""
	tempMu.Unlock()
}

// cleanupTempFile removes the tracked temp file if set.
// R1.5: called on signal receipt and on error exit.
func cleanupTempFile() {
	tempMu.Lock()
	p := tempFilePath
	tempFilePath = ""
	tempMu.Unlock()
	if p != "" {
		os.Remove(p) // best-effort cleanup
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
func writeToStdout(data []byte, tmpPath string) error {
	if tmpPath != "" {
		return copyFileToWriter(tmpPath, os.Stdout)
	}
	_, err := os.Stdout.Write(data)
	return err
}

// writeToFile writes buffered content to the named output file.
// R2.1: attempts atomic rename of temp file to output path.
// R2.2: falls back to byte-for-byte copy when rename fails.
// R2.3: preserves file mode of existing output file.
func writeToFile(outFile string, data []byte, tmpPath string) error {
	mode := getFileMode(outFile)
	tmpPath, err := ensureTempFile(data, tmpPath, outFile)
	if err != nil {
		return err
	}
	// R2.3: apply target mode to temp file before rename.
	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}
	// R2.1: attempt atomic rename.
	if err := os.Rename(tmpPath, outFile); err != nil {
		return fallbackCopy(tmpPath, outFile, mode)
	}
	clearTempFile()
	return nil
}

// getFileMode returns the permission bits of an existing file, or 0666 for new files.
// R2.3: uses Lstat to read mode without following symlinks.
func getFileMode(path string) os.FileMode {
	info, err := os.Lstat(path)
	if err != nil {
		return 0o666
	}
	return info.Mode().Perm()
}

// ensureTempFile creates a temp file from in-memory data if needed.
// When tmpPath is already set (spill case), returns it unchanged.
// When data is in memory, writes to a new temp file in the output directory.
func ensureTempFile(data []byte, tmpPath, outFile string) (string, error) {
	if tmpPath != "" {
		return tmpPath, nil
	}
	dir := filepath.Dir(outFile)
	f, err := os.CreateTemp(dir, "sponge.")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	p := f.Name()
	setTempFile(p)
	if _, err := f.Write(data); err != nil {
		f.Close()    // best-effort close
		os.Remove(p) // best-effort cleanup
		return "", fmt.Errorf("writing temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(p)
		return "", fmt.Errorf("closing temp file: %w", err)
	}
	return p, nil
}

// fallbackCopy copies temp file to output when rename fails.
// R2.2: used for cross-device moves or other rename failures.
// R2.3: applies the specified mode to the output file.
func fallbackCopy(tmpPath, outFile string, mode os.FileMode) error {
	f, err := os.OpenFile(outFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("opening %s: %w", outFile, err)
	}
	defer f.Close()
	if err := copyFileToWriter(tmpPath, f); err != nil {
		return err
	}
	os.Remove(tmpPath) // best-effort cleanup after successful copy
	clearTempFile()
	return nil
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
