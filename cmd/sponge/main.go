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
	"runtime"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

var appendMode = flag.Bool("a", false, "append to output file")

func main() {
	sys.InstallSIGPIPEHandler()
	flag.Parse()
	os.Exit(run())
}

func run() int {
	data, tmpFile, err := readAllStdin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %v\n", err)
		return 1
	}
	if tmpFile != "" {
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

func writeOutputFile(filename string, data []byte, tmpFile string) error {
	return nil
}

func prependOriginalContent(filename string, data []byte) ([]byte, error) {
	return nil, nil
}

func writeToStdout(data []byte, tmpFile string) error {
	return nil
}

func cleanupTempFile(path string) {
}
