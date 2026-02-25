// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the sponge utility: soak stdin and write to file.
//
// Implements: prd007-sponge (R1-R5)
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

// initialBufSize is the starting buffer size for stdin soaking.
// Per prd007-sponge R1.2: starts at 8192 bytes.
const initialBufSize = 8192

// tempFilePrefix is the prefix for temporary files.
// Per prd007-sponge R1.4 and design decision D3.
const tempFilePrefix = "sponge"

// options holds the parsed command-line flags for sponge.
type options struct {
	append bool // -a: append mode (prepend original file content before stdin content).
}

func main() {
	// SIGPIPE handling: exit 0 silently on broken pipe.
	// Per prd007-sponge and design decision D2, matching cmd/cat pattern.
	sigpipeCh := make(chan os.Signal, 1)
	signal.Notify(sigpipeCh, syscall.SIGPIPE)
	go func() {
		<-sigpipeCh
		os.Exit(0)
	}()

	opts, outputFile, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %s\n", err)
		os.Exit(1)
	}

	if err := run(opts, outputFile); err != nil {
		fmt.Fprintf(os.Stderr, "sponge: %s\n", err)
		os.Exit(1)
	}
}

// parseFlags parses sponge command-line arguments using manual flag parsing.
// Per design decision D1: iterate over os.Args[1:], handle '-a' as append flag,
// treat '--' as end of flags, remaining positional arg is the output filename.
func parseFlags(args []string) (options, string, error) {
	var opts options
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'a':
					// Per prd007-sponge R3: append mode.
					opts.append = true
				default:
					return options{}, "", fmt.Errorf("invalid option -- '%c'", ch)
				}
			}
		} else {
			files = append(files, arg)
		}
	}

	var outputFile string
	if len(files) > 0 {
		outputFile = files[0]
	}

	return opts, outputFile, nil
}

// run executes the sponge logic: soak stdin, then write to output.
func run(opts options, outputFile string) error {
	// Soak all of stdin. Per prd007-sponge R1.1: read all bytes before
	// opening the output file.
	buf, tmpFile, err := soakStdin()
	if err != nil {
		if tmpFile != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name()) // best-effort cleanup
		}
		return fmt.Errorf("reading stdin: %w", err)
	}

	// Register cleanup for the temp file if one was created.
	// Per prd007-sponge R1.5.
	if tmpFile != nil {
		registerCleanup(tmpFile.Name())
	}

	if outputFile == "" {
		// Passthrough mode. Per prd007-sponge R4.1-R4.3.
		err = writePassthrough(buf, tmpFile)
		if tmpFile != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name()) // best-effort cleanup
		}
		return err
	}

	// Output file mode. Per prd007-sponge R2.
	err = writeOutputFile(buf, tmpFile, outputFile, opts.append)
	if tmpFile != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name()) // best-effort cleanup
	}
	return err
}

// soakStdin reads all of stdin into memory. If the buffer exceeds the memory
// threshold, it spills to a temp file and continues reading.
//
// Returns the in-memory buffer (may be nil if spilled) and a temp file (may be
// nil if everything fit in memory).
//
// Per prd007-sponge R1.1, R1.2, R1.3, R1.4.
func soakStdin() ([]byte, *os.File, error) {
	buf := make([]byte, 0, initialBufSize)
	threshold := memoryThreshold()

	chunk := make([]byte, initialBufSize)
	var tmpFile *os.File

	for {
		n, readErr := os.Stdin.Read(chunk)
		if n > 0 {
			if tmpFile != nil {
				// Already spilled: write directly to temp file.
				if _, err := tmpFile.Write(chunk[:n]); err != nil {
					return nil, tmpFile, fmt.Errorf("writing to temp file: %w", err)
				}
			} else {
				buf = append(buf, chunk[:n]...)
				// Check if we need to spill.
				// Per prd007-sponge R1.3.
				if int64(len(buf)) > threshold {
					var err error
					tmpFile, err = createTempFile()
					if err != nil {
						return nil, nil, fmt.Errorf("creating temp file: %w", err)
					}
					// Write accumulated buffer to temp file.
					if _, err := tmpFile.Write(buf); err != nil {
						return nil, tmpFile, fmt.Errorf("spilling to temp file: %w", err)
					}
					buf = nil // Release memory.
				}
			}
		}

		if readErr == io.EOF {
			return buf, tmpFile, nil
		}
		if readErr != nil {
			return buf, tmpFile, readErr
		}
	}
}

// memoryThreshold returns the memory threshold above which stdin content
// is spilled to a temp file. Per prd007-sponge R1.3: derived from available
// memory. We use a fraction of the system memory as reported by runtime.
func memoryThreshold() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// Use half of system memory as the threshold, with a minimum of 64 MB.
	threshold := int64(m.Sys) / 2
	const minThreshold = 64 * 1024 * 1024
	if threshold < minThreshold {
		threshold = minThreshold
	}
	return threshold
}

// createTempFile creates a temp file in the directory specified by TMPDIR,
// or the system default temp directory if TMPDIR is unset.
// Per prd007-sponge R1.4 and design decision D3.
func createTempFile() (*os.File, error) {
	dir := os.Getenv("TMPDIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return os.CreateTemp(dir, tempFilePrefix)
}

// registerCleanup registers signal handlers that delete the temp file if the
// process receives SIGINT, SIGTERM, SIGHUP, or SIGPIPE before completion.
// Per prd007-sponge R1.5.
func registerCleanup(tmpPath string) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-ch
		os.Remove(tmpPath) // best-effort cleanup
		os.Exit(1)
	}()
}

// writePassthrough writes the soaked stdin to stdout.
// Per prd007-sponge R4.1-R4.3.
func writePassthrough(buf []byte, tmpFile *os.File) error {
	if tmpFile != nil {
		// Per prd007-sponge R4.2: seek to beginning and copy to stdout.
		if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("seeking temp file: %w", err)
		}
		if _, err := io.Copy(os.Stdout, tmpFile); err != nil {
			return fmt.Errorf("writing to stdout: %w", err)
		}
		return nil
	}

	// Per prd007-sponge R4.3: write in-memory buffer directly to stdout.
	if len(buf) > 0 {
		if _, err := os.Stdout.Write(buf); err != nil {
			return fmt.Errorf("writing to stdout: %w", err)
		}
	}
	return nil
}

// writeOutputFile writes the soaked stdin to the output file, handling atomic
// write via temp file rename, append mode, and file mode preservation.
//
// Per prd007-sponge R2.1-R2.5, R3.1-R3.3.
func writeOutputFile(buf []byte, spillFile *os.File, outputPath string, appendMode bool) error {
	// Per design decision D5: use os.Lstat (not os.Stat) to check output path.
	// Per prd007-sponge R2.3, R2.4.
	var existingMode os.FileMode
	var outputExists bool

	info, err := os.Lstat(outputPath)
	if err == nil {
		outputExists = true
		existingMode = info.Mode()
	}

	// Create a temp file in the same directory as the output file for atomic rename.
	// Per prd007-sponge R2.1.
	outDir := dirOfPath(outputPath)
	outTmp, err := os.CreateTemp(outDir, tempFilePrefix)
	if err != nil {
		return fmt.Errorf("creating output temp file: %w", err)
	}
	outTmpPath := outTmp.Name()

	// Ensure cleanup of the output temp file on error.
	success := false
	defer func() {
		if !success {
			outTmp.Close()
			os.Remove(outTmpPath) // best-effort cleanup
		}
	}()

	// Per prd007-sponge R3.1-R3.3: in append mode, prepend original file content.
	if appendMode && outputExists && info.Mode().IsRegular() {
		// Per prd007-sponge R3.3: copy original file content into temp file first.
		origFile, err := os.Open(outputPath)
		if err != nil {
			return fmt.Errorf("reading original file for append: %w", err)
		}
		if _, err := io.Copy(outTmp, origFile); err != nil {
			origFile.Close()
			return fmt.Errorf("copying original content for append: %w", err)
		}
		origFile.Close()
	}

	// Write the stdin content to the output temp file.
	if spillFile != nil {
		if _, err := spillFile.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("seeking spill file: %w", err)
		}
		if _, err := io.Copy(outTmp, spillFile); err != nil {
			return fmt.Errorf("writing spill content to output: %w", err)
		}
	} else if len(buf) > 0 {
		if _, err := outTmp.Write(buf); err != nil {
			return fmt.Errorf("writing buffer to output: %w", err)
		}
	}

	// Close the temp file before rename.
	if err := outTmp.Close(); err != nil {
		return fmt.Errorf("closing output temp file: %w", err)
	}

	// Apply file mode. Per prd007-sponge R2.3.
	if outputExists {
		if err := os.Chmod(outTmpPath, existingMode); err != nil {
			return fmt.Errorf("setting file mode: %w", err)
		}
	}
	// When the output file does not exist, os.CreateTemp uses 0600 by default.
	// The PRD specifies 0666 masked by umask. Apply it explicitly for new files.
	if !outputExists {
		// Get umask by temporarily setting it and restoring.
		umask := syscall.Umask(0)
		syscall.Umask(umask)
		if err := os.Chmod(outTmpPath, os.FileMode(0666&^umask)); err != nil {
			return fmt.Errorf("setting file mode for new file: %w", err)
		}
	}

	// Attempt atomic rename. Per prd007-sponge R2.1, design decision D4.
	if err := os.Rename(outTmpPath, outputPath); err != nil {
		// Per prd007-sponge R2.2 and design decision D4: cross-device fallback.
		if isCrossDeviceError(err) {
			if err := crossDeviceCopy(outTmpPath, outputPath, existingMode, outputExists); err != nil {
				return err
			}
			os.Remove(outTmpPath) // best-effort cleanup of temp file
			success = true
			return nil
		}
		return fmt.Errorf("renaming temp file to output: %w", err)
	}

	success = true
	return nil
}

// dirOfPath returns the directory component of a path. Returns "." if the path
// has no directory separator.
func dirOfPath(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				return "/"
			}
			return path[:i]
		}
	}
	return "."
}

// isCrossDeviceError checks if an error from os.Rename indicates a cross-device
// link failure. Per design decision D4.
func isCrossDeviceError(err error) bool {
	linkErr, ok := err.(*os.LinkError)
	if !ok {
		return false
	}
	return linkErr.Err == syscall.EXDEV
}

// crossDeviceCopy performs a byte-for-byte copy as a fallback when atomic rename
// fails due to cross-device link. Per prd007-sponge R2.2.
func crossDeviceCopy(srcPath string, dstPath string, mode os.FileMode, dstExists bool) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("opening temp file for cross-device copy: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("creating output file for cross-device copy: %w", err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return fmt.Errorf("cross-device copy: %w", err)
	}

	if err := dst.Close(); err != nil {
		return fmt.Errorf("closing output file after cross-device copy: %w", err)
	}

	if dstExists {
		if err := os.Chmod(dstPath, mode); err != nil {
			return fmt.Errorf("setting mode after cross-device copy: %w", err)
		}
	}

	return nil
}
