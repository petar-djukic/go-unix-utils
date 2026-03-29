// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sponge implements moreutils sponge: soak stdin before writing output.
//
// Implements prd007-sponge R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3, R2.4, R2.5, R3.1, R3.2, R3.3, R4.1, R4.2, R4.3, R5.1, R5.2, R5.3, R5.4.
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// initialBufSize matches sponge.c BUFF_SIZE (R1.2).
const initialBufSize = 8192

// tempPrefix is the mkstemp-style prefix for temp files (R1.4).
const tempPrefix = "sponge."

func main() {
	sys.InstallSIGPIPEHandler()

	// R5.1: exit 0 on success; R5.2: exit 1 on errors.
	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run implements the sponge logic: read all stdin, then write output.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	appendMode, outFile := parseArgs(args)

	buf, tmpPath, err := soakStdin(stdin)
	if tmpPath != "" {
		defer os.Remove(tmpPath) // R5.4: best-effort cleanup on all paths
		// R1.5: register signal handler to clean up temp file on termination
		stopCleanup := setupSignalCleanup(tmpPath)
		defer stopCleanup()
	}
	if err != nil {
		// R5.2: print descriptive error to stderr and exit 1
		fmt.Fprintf(stderr, "sponge: %v\n", err)
		return 1
	}

	if outFile == "" {
		return writeToStdout(buf, tmpPath, stdout, stderr)
	}
	return writeToFile(buf, tmpPath, outFile, appendMode, stderr)
}

// parseArgs extracts the -a flag and output filename from arguments.
func parseArgs(args []string) (bool, string) {
	appendMode := false
	outFile := ""
	for _, arg := range args {
		if arg == "-a" {
			appendMode = true
		} else {
			outFile = arg
		}
	}
	return appendMode, outFile
}

// setupSignalCleanup registers a handler to delete tmpPath on termination signals (R1.5, R5.4).
func setupSignalCleanup(tmpPath string) func() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	done := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			os.Remove(tmpPath) // R5.4: best-effort cleanup before exit
			os.Exit(1)
		case <-done:
		}
	}()
	return func() {
		signal.Stop(sigCh)
		close(done)
	}
}

// soakStdin reads all of stdin into memory or spills to a temp file (R1.1, R1.2, R1.3).
// Returns the in-memory buffer (nil if spilled), the temp file path (empty if in-memory), and any error.
// R5.3: recovers from allocation panics instead of crashing.
func soakStdin(stdin io.Reader) (buf []byte, tmpPath string, err error) {
	defer func() {
		// R5.3: recover from memory allocation failure (panic from make/append)
		if r := recover(); r != nil {
			err = fmt.Errorf("memory allocation failed: %v", r)
		}
	}()

	buf = make([]byte, 0, initialBufSize)
	threshold := memoryThreshold()

	for {
		if cap(buf)-len(buf) == 0 {
			newCap := max(cap(buf)*2, initialBufSize)
			grown := make([]byte, len(buf), newCap)
			copy(grown, buf)
			buf = grown
		}

		n, readErr := stdin.Read(buf[len(buf):cap(buf)])
		buf = buf[:len(buf)+n]

		if uint64(len(buf)) > threshold {
			return spillToTempFile(buf, stdin)
		}

		if readErr == io.EOF {
			return buf, "", nil
		}
		if readErr != nil {
			// R5.2: stdin read error
			return nil, "", fmt.Errorf("read stdin: %w", readErr)
		}
	}
}

// memoryThreshold returns the byte threshold at which stdin spills to a temp file (R1.3).
func memoryThreshold() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	avail := m.Sys - m.HeapInuse
	if avail < initialBufSize*4 {
		avail = 64 * 1024 * 1024 // 64 MB fallback
	}
	return avail / 2
}

// createTempFile creates a temp file in TMPDIR or /tmp (R1.4).
func createTempFile() (*os.File, string, error) {
	tmpDir := os.Getenv("TMPDIR") // platform context: temp directory
	if tmpDir == "" {
		tmpDir = "/tmp"
	}
	f, err := os.CreateTemp(tmpDir, tempPrefix)
	if err != nil {
		// R5.2: temp file creation error
		return nil, "", fmt.Errorf("create temp file: %w", err)
	}
	return f, f.Name(), nil
}

// spillToTempFile writes the current buffer and remaining stdin to a temp file (R1.3, R1.4).
func spillToTempFile(buf []byte, remaining io.Reader) ([]byte, string, error) {
	f, tmpPath, err := createTempFile()
	if err != nil {
		return nil, "", err
	}

	if _, err := f.Write(buf); err != nil {
		f.Close() // best-effort close before cleanup
		// R5.4: tmpPath returned so caller can clean up
		return nil, tmpPath, fmt.Errorf("write to temp file: %w", err)
	}

	if _, err := io.Copy(f, remaining); err != nil {
		f.Close() // best-effort close before cleanup
		return nil, tmpPath, fmt.Errorf("copy stdin to temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		return nil, tmpPath, fmt.Errorf("close temp file: %w", err)
	}

	return nil, tmpPath, nil
}

// writeToStdout writes the buffered content to stdout (R4.1, R4.2, R4.3).
func writeToStdout(buf []byte, tmpPath string, stdout io.Writer, stderr io.Writer) int {
	if tmpPath != "" {
		if err := copyFileToWriter(tmpPath, stdout); err != nil {
			fmt.Fprintf(stderr, "sponge: %v\n", err)
			return 1
		}
		return 0 // R5.1: success
	}
	if _, err := stdout.Write(buf); err != nil {
		fmt.Fprintf(stderr, "sponge: %v\n", err)
		return 1
	}
	return 0 // R5.1: success
}

// writeToFile writes the buffered content to the named output file (R2.1-R2.5, R3.1, R3.2).
func writeToFile(buf []byte, tmpPath string, outFile string, appendMode bool, stderr io.Writer) int {
	// R2.4: use lstat to check file existence and type
	mode, isReg := fileStatus(outFile)

	// R3.1, R3.2: prepend original content in append mode for existing regular files
	if appendMode && isReg {
		var err error
		buf, tmpPath, err = prependOriginal(buf, tmpPath, outFile)
		if err != nil {
			fmt.Fprintf(stderr, "sponge: %v\n", err)
			return 1
		}
		if tmpPath != "" {
			defer os.Remove(tmpPath) // R5.4: best-effort cleanup of append temp
		}
	}

	return finishWrite(buf, tmpPath, outFile, mode, stderr)
}

// finishWrite completes the write to outFile via rename or direct write (R2.1, R2.2, R2.5).
func finishWrite(buf []byte, tmpPath string, outFile string, mode os.FileMode, stderr io.Writer) int {
	if tmpPath != "" {
		// R2.1: attempt atomic rename; R2.2: fall back to copy on failure
		if err := renameOrCopy(tmpPath, outFile); err != nil {
			// R5.2: rename/copy failure
			fmt.Fprintf(stderr, "sponge: %v\n", err)
			return 1
		}
	} else {
		if err := os.WriteFile(outFile, buf, mode); err != nil {
			// R5.2: output file open/write failure
			fmt.Fprintf(stderr, "sponge: %v\n", err)
			return 1
		}
	}
	// R2.3: apply preserved permissions (or default 0666 for new files)
	if err := os.Chmod(outFile, mode); err != nil {
		fmt.Fprintf(stderr, "sponge: %v\n", err)
		return 1
	}
	return 0 // R5.1: success
}

// fileStatus returns file permissions and whether path is a regular file via lstat (R2.3, R2.4).
// Returns 0o666 and false when the file does not exist.
func fileStatus(path string) (os.FileMode, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0o666, false
	}
	return info.Mode().Perm(), info.Mode().IsRegular()
}

// prependOriginal prepends the original file content to the stdin content (R3.1, R3.3).
func prependOriginal(buf []byte, tmpPath string, outFile string) ([]byte, string, error) {
	origData, err := os.ReadFile(outFile)
	if err != nil {
		return nil, tmpPath, fmt.Errorf("read original file: %w", err)
	}

	if tmpPath == "" {
		// In-memory: combine original + stdin buffer
		combined := make([]byte, len(origData)+len(buf))
		copy(combined, origData)
		copy(combined[len(origData):], buf)
		return combined, "", nil
	}

	return prependToTempFile(origData, tmpPath)
}

// prependToTempFile creates a new temp file with original data followed by stdin temp content (R3.3).
func prependToTempFile(origData []byte, stdinTmpPath string) ([]byte, string, error) {
	f, newPath, err := createTempFile()
	if err != nil {
		return nil, "", err
	}

	if _, err := f.Write(origData); err != nil {
		f.Close() // best-effort close
		return nil, newPath, fmt.Errorf("write original to temp: %w", err)
	}

	if err := copyFileToWriter(stdinTmpPath, f); err != nil {
		f.Close() // best-effort close
		return nil, newPath, err
	}

	if err := f.Close(); err != nil {
		return nil, newPath, fmt.Errorf("close temp file: %w", err)
	}
	return nil, newPath, nil
}

// renameOrCopy attempts an atomic rename; falls back to copy on failure (R2.1, R2.2, R2.5).
func renameOrCopy(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	return copyFile(src, dst)
}

// copyFile copies src to dst by reading and writing content.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create output file %s: %w", filepath.Base(dst), err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy to output file: %w", err)
	}
	return out.Close()
}

// copyFileToWriter copies the content of the file at path to w.
func copyFileToWriter(path string, w io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("write to stdout: %w", err)
	}
	return nil
}
