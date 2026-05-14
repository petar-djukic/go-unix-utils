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

func run() int {
	installCleanupHandler()

	data, tmpFile, err := readAllStdin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		return 1
	}
	if tmpFile != "" {
		setTempFile(tmpFile)
		defer cleanupTempFile(tmpFile)
	}

	args := flag.Args()
	if len(args) == 0 {
		if err := writeToStdout(data, tmpFile); err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			return 1
		}
		return 0
	}

	filename := args[0]
	if *appendMode {
		data, err = prependOriginalContent(filename, data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
			return 1
		}
	}

	if err := writeOutputFile(filename, data, tmpFile); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		return 1
	}
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

func writeOutputFile(filename string, data []byte, tmpFile string) error {
	mode, exists, err := outputFileMode(filename)
	if err != nil {
		return err
	}

	tmpPath := tmpFile
	if tmpPath == "" {
		tmpPath, err = spillToTempFile(data)
		if err != nil {
			return err
		}
		setTempFile(tmpPath)
		defer func() {
			cleanupTempFile(tmpPath)
			clearTempFile()
		}()
	}

	if err := attemptAtomicWrite(tmpPath, filename, mode, exists); err != nil {
		return err
	}
	clearTempFile()
	return nil
}

func outputFileMode(filename string) (os.FileMode, bool, error) {
	fi, err := sys.Lstat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return 0666, false, nil
		}
		return 0, false, fmt.Errorf("lstat %s: %w", filename, err)
	}
	return fi.Mode.Perm(), true, nil
}

func attemptAtomicWrite(tmpPath, filename string, mode os.FileMode, exists bool) error {
	if err := os.Rename(tmpPath, filename); err != nil {
		return fallbackCopy(tmpPath, filename, mode)
	}
	if exists {
		if err := os.Chmod(filename, mode); err != nil {
			return fmt.Errorf("chmod %s: %w", filename, err)
		}
	}
	return nil
}

func fallbackCopy(tmpPath, filename string, mode os.FileMode) error {
	src, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("open %s: %w", filename, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy to %s: %w", filename, err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("close %s: %w", filename, err)
	}
	src.Close()
	os.Remove(tmpPath)
	return nil
}

func prependOriginalContent(filename string, data []byte) ([]byte, error) {
	original, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, fmt.Errorf("read %s: %w", filename, err)
	}
	combined := make([]byte, len(original)+len(data))
	copy(combined, original)
	copy(combined[len(original):], data)
	return combined, nil
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

func cleanupTempFile(path string) {
	os.Remove(path)
}
