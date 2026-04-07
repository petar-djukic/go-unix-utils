// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the sponge utility.
// Implements srd007-sponge R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3, R2.4, R2.5, R3.1, R3.2, R3.3, R4.1, R4.2, R4.3, R5.1, R5.2, R5.3, R5.4.
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

// R5.1: exits 0 on success (implicit Go behavior when main returns).
// R5.2: exits 1 with descriptive stderr message on any error.
// R5.3: recovers from allocation panics and exits 1 instead of crashing.
// R5.4: cleans up temp file on all exit paths.
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
// R5.3: deferred recover converts allocation panics to errors.
// R5.4: deferred cleanupTempFile ensures temp file removal on all paths.
func run() (retErr error) {
	// R5.3: recover from memory allocation panics.
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("memory allocation failed: %v", r)
		}
	}()

	appendMode, outFile := parseArgs()

	data, tmpPath, err := readAllStdin()
	setTempFile(tmpPath)
	defer cleanupTempFile()

	if err != nil {
		return err
	}
	return writeOutput(outFile, data, tmpPath, appendMode)
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
// R4.1: when no output filename is given, writes buffered stdin to stdout.
func writeOutput(outFile string, data []byte, tmpPath string, appendMode bool) error {
	if outFile == "" {
		return writeToStdout(data, tmpPath)
	}
	return writeToFile(outFile, data, tmpPath, appendMode)
}

// writeToStdout writes buffered content to stdout.
// R4.2: for large input (temp file spill), copies temp file content to stdout.
// R4.3: for small input (in-memory), writes buffer directly to stdout.
func writeToStdout(data []byte, tmpPath string) error {
	if tmpPath != "" {
		return copyFileToWriter(tmpPath, os.Stdout)
	}
	_, err := os.Stdout.Write(data)
	return err
}

// writeToFile writes buffered content to the named output file.
// R2.1: attempts atomic rename of temp file to output path.
// R2.2: falls back to atomic copy via temp-in-output-dir when rename fails.
// R2.3: preserves file mode of existing output file.
// R2.4: uses lstat to check output path type.
// R2.5: ensures output is never in a partially-written state.
// R3.1/R3.2: prepends existing content in append mode for regular files.
func writeToFile(outFile string, data []byte, tmpPath string, appendMode bool) error {
	// R2.4: use lstat to determine file type and mode.
	mode, isRegular := getFileModeAndType(outFile)
	tmpPath, err := ensureTempFile(data, tmpPath, outFile)
	if err != nil {
		return err
	}
	// R3.1/R3.2: prepend existing file content in append mode.
	if appendMode && isRegular {
		tmpPath, err = prependExistingContent(outFile, tmpPath)
		if err != nil {
			return err
		}
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := os.Rename(tmpPath, outFile); err != nil {
		return fallbackCopy(tmpPath, outFile, mode)
	}
	clearTempFile()
	return nil
}

// getFileModeAndType returns the permission bits and whether the path
// is a regular file. Returns (0666, false) for nonexistent paths.
// R2.4: uses lstat to avoid following symlinks.
func getFileModeAndType(path string) (os.FileMode, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0o666, false
	}
	return info.Mode().Perm(), info.Mode().IsRegular()
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

// prependExistingContent creates a new temp file with the existing
// output file content followed by the stdin content from the old temp file.
// R3.1: result is [original file content][stdin content].
// R3.3: original file is read before temp file is renamed.
func prependExistingContent(outFile, tmpPath string) (string, error) {
	dir := filepath.Dir(tmpPath)
	combined, err := os.CreateTemp(dir, "sponge.")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	combinedPath := combined.Name()
	if err := copyFileToWriter(outFile, combined); err != nil {
		combined.Close()
		os.Remove(combinedPath)
		return "", fmt.Errorf("reading original file: %w", err)
	}
	if err := copyFileToWriter(tmpPath, combined); err != nil {
		combined.Close()
		os.Remove(combinedPath)
		return "", err
	}
	if err := combined.Close(); err != nil {
		os.Remove(combinedPath)
		return "", fmt.Errorf("closing temp file: %w", err)
	}
	os.Remove(tmpPath) // best-effort cleanup of old temp
	setTempFile(combinedPath)
	return combinedPath, nil
}

// fallbackCopy creates a temp file in the output directory, copies content
// to it, then renames atomically. Used when direct rename fails (cross-device).
// R2.2: byte-for-byte copy when rename fails.
// R2.5: output file is never in a partially-written state.
func fallbackCopy(tmpPath, outFile string, mode os.FileMode) error {
	dir := filepath.Dir(outFile)
	localTmp, err := os.CreateTemp(dir, "sponge.")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	localPath := localTmp.Name()
	if err := copyFileToWriter(tmpPath, localTmp); err != nil {
		localTmp.Close()
		os.Remove(localPath)
		return err
	}
	if err := localTmp.Close(); err != nil {
		os.Remove(localPath)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(localPath, mode); err != nil {
		os.Remove(localPath)
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := os.Rename(localPath, outFile); err != nil {
		os.Remove(localPath)
		return fmt.Errorf("renaming to %s: %w", outFile, err)
	}
	os.Remove(tmpPath) // best-effort cleanup of original temp
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
