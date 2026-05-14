// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sponge implements srd007-sponge: soak all of stdin before writing
// to a named output file, preventing read-write conflicts in pipelines.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

var appendMode = flag.Bool("a", false, "append to output file")

var (
	tempFileMu   sync.Mutex
	tempFilePath string
)

func main() {
	sys.InstallSIGPIPEHandler()
	flag.Parse()
	os.Exit(run())
}

func run() (exitCode int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", r)
			exitCode = 1
		}
	}()
	installCleanupHandler()
	defer cleanupCurrentTempFile()

	data, tmpFile, err := readAllStdin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		return 1
	}
	if tmpFile != "" {
		setTempFile(tmpFile)
	}

	args := flag.Args()
	if len(args) == 0 {
		return runPassthrough(data, tmpFile)
	}
	return runOutput(args[0], data, tmpFile)
}

func runPassthrough(data []byte, tmpFile string) int {
	if err := writeToStdout(data, tmpFile); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		return 1
	}
	return 0
}

func runOutput(filename string, data []byte, tmpFile string) int {
	mode, exists, isRegular, err := outputFileInfo(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		return 1
	}

	// R3.1, R3.2: append only when file exists and is a regular file
	if *appendMode && exists && isRegular {
		data, tmpFile, err = prependOriginal(filename, data, tmpFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			return 1
		}
		if tmpFile != "" {
			setTempFile(tmpFile)
		}
	}

	if err := writeOutputFile(filename, data, tmpFile, mode, exists, isRegular); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		return 1
	}
	clearTempFile()
	return 0
}

func readAllStdin() ([]byte, string, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, "", fmt.Errorf("read stdin: %w", err)
	}
	threshold := spillThreshold()
	if uint64(len(data)) > threshold {
		tmpPath, err := spillToTempFile(data)
		if err != nil {
			return nil, "", err
		}
		return nil, tmpPath, nil
	}
	return data, "", nil
}

func spillThreshold() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	avail := m.Sys - m.HeapInuse
	if avail == 0 {
		return 256 * 1024 * 1024
	}
	quarter := avail / 4
	const maxThreshold = 1024 * 1024 * 1024
	if quarter > maxThreshold {
		return maxThreshold
	}
	return quarter
}

func spillToTempFile(data []byte) (string, error) {
	f, err := os.CreateTemp(tempDir(), "sponge.")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	path := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(path)
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("close temp file: %w", err)
	}
	return path, nil
}

func tempDir() string {
	if d := os.Getenv("TMPDIR"); d != "" {
		return d
	}
	return "/tmp"
}

func setTempFile(path string) {
	tempFileMu.Lock()
	tempFilePath = path
	tempFileMu.Unlock()
}

func clearTempFile() {
	tempFileMu.Lock()
	tempFilePath = ""
	tempFileMu.Unlock()
}

func cleanupCurrentTempFile() {
	tempFileMu.Lock()
	p := tempFilePath
	tempFilePath = ""
	tempFileMu.Unlock()
	if p != "" {
		os.Remove(p)
	}
}

func installCleanupHandler() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)
	go func() {
		<-sigCh
		tempFileMu.Lock()
		p := tempFilePath
		tempFileMu.Unlock()
		if p != "" {
			os.Remove(p)
		}
		os.Exit(1)
	}()
}

// R2.4: use lstat to determine file existence, mode, and whether it is regular
func outputFileInfo(filename string) (os.FileMode, bool, bool, error) {
	fi, err := sys.Lstat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return 0666, false, false, nil
		}
		return 0, false, false, fmt.Errorf("lstat %s: %w", filename, err)
	}
	return fi.Mode.Perm(), true, fi.Mode.IsRegular(), nil
}

// R3.1: prepend original file content before stdin content
func prependOriginal(filename string, data []byte, tmpFile string) ([]byte, string, error) {
	original, err := os.ReadFile(filename)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", filename, err)
	}
	if len(original) == 0 {
		return data, tmpFile, nil
	}
	if tmpFile != "" {
		combined, err := combineTempFiles(original, tmpFile)
		if err != nil {
			return nil, "", err
		}
		return nil, combined, nil
	}
	combined := make([]byte, len(original)+len(data))
	copy(combined, original)
	copy(combined[len(original):], data)
	return combined, "", nil
}

func combineTempFiles(original []byte, stdinTmpFile string) (string, error) {
	f, err := os.CreateTemp(tempDir(), "sponge.")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	path := f.Name()

	if _, err := f.Write(original); err != nil {
		f.Close()
		os.Remove(path)
		return "", fmt.Errorf("write original to temp: %w", err)
	}
	if err := appendFromFile(f, stdinTmpFile); err != nil {
		f.Close()
		os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("close temp file: %w", err)
	}

	setTempFile(path)
	os.Remove(stdinTmpFile)
	return path, nil
}

func appendFromFile(dst *os.File, srcPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", srcPath, err)
	}
	defer src.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy from %s: %w", srcPath, err)
	}
	return nil
}

func writeOutputFile(filename string, data []byte, tmpFile string, mode os.FileMode, exists bool, isRegular bool) error {
	tmpPath := tmpFile
	var err error
	if tmpPath == "" {
		tmpPath, err = spillToTempFile(data)
		if err != nil {
			return err
		}
		setTempFile(tmpPath)
	}

	// R2.4: only attempt atomic rename for regular files or new files
	if !exists || isRegular {
		return attemptAtomicWrite(tmpPath, filename, mode, exists)
	}
	return directWrite(tmpPath, filename, mode)
}

func attemptAtomicWrite(tmpPath, filename string, mode os.FileMode, exists bool) error {
	if err := os.Rename(tmpPath, filename); err != nil {
		return atomicFallbackCopy(tmpPath, filename, mode)
	}
	if exists {
		if err := os.Chmod(filename, mode); err != nil {
			return fmt.Errorf("chmod %s: %w", filename, err)
		}
	}
	return nil
}

// R2.5: ensure output is never in a partially-written state
func atomicFallbackCopy(tmpPath, filename string, mode os.FileMode) error {
	dir := filepath.Dir(filename)
	tmpDst, err := os.CreateTemp(dir, ".sponge-")
	if err != nil {
		return directWrite(tmpPath, filename, mode)
	}
	tmpDstPath := tmpDst.Name()

	if err := copyFileToWriter(tmpPath, tmpDst, mode); err != nil {
		os.Remove(tmpDstPath)
		return err
	}
	if err := os.Rename(tmpDstPath, filename); err != nil {
		os.Remove(tmpDstPath)
		return directWrite(tmpPath, filename, mode)
	}
	os.Remove(tmpPath)
	return nil
}

func copyFileToWriter(srcPath string, dst *os.File, mode os.FileMode) error {
	src, err := os.Open(srcPath)
	if err != nil {
		dst.Close()
		return fmt.Errorf("open temp file: %w", err)
	}
	defer src.Close()

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return fmt.Errorf("copy: %w", err)
	}
	if err := dst.Chmod(mode); err != nil {
		dst.Close()
		return fmt.Errorf("chmod: %w", err)
	}
	return dst.Close()
}

func directWrite(tmpPath, filename string, mode os.FileMode) error {
	src, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("open %s: %w", filename, err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return fmt.Errorf("copy to %s: %w", filename, err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("close %s: %w", filename, err)
	}
	os.Remove(tmpPath)
	return nil
}

func writeToStdout(data []byte, tmpFile string) error {
	if tmpFile != "" {
		return copyTempFileToStdout(tmpFile)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		return fmt.Errorf("write stdout: %w", err)
	}
	return nil
}

func copyTempFileToStdout(tmpFile string) error {
	f, err := os.Open(tmpFile)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	defer f.Close()
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek temp file: %w", err)
	}
	if _, err := io.Copy(os.Stdout, f); err != nil {
		return fmt.Errorf("write stdout: %w", err)
	}
	return nil
}
