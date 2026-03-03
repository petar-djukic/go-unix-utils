// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sponge reads all of stdin into a buffer before writing to the output
// file, enabling safe in-place file modification in pipelines like
// "cmd file | sponge file". Optionally appends to the output file with -a.
//
// Implements prd007-sponge R1, R2, R3, R4, R5.
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName       = "sponge"
	initialBufSize = 8192
)

type config struct {
	appendMode bool // -a
}

var (
	tempMu    sync.Mutex
	tempPaths []string
)

func main() {
	sys.InstallSIGPIPEHandler()
	installCleanupHandler()

	cfg, outputFile := parseArgs(os.Args[1:])

	// Phase 1: Read all of stdin before touching the output file (R1.1).
	buf, soakTmpPath, err := soakStdin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		cleanupAllTempFiles()
		os.Exit(1)
	}

	// Phase 2: Write output.
	if outputFile == "" {
		// Passthrough mode: write to stdout (R4).
		if err := writePassthrough(buf, soakTmpPath); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			cleanupAllTempFiles()
			os.Exit(1)
		}
	} else {
		// File output mode (R2, R3).
		if err := writeToFile(cfg, buf, soakTmpPath, outputFile); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			cleanupAllTempFiles()
			os.Exit(1)
		}
	}

	cleanupAllTempFiles()
}

// parseArgs parses sponge flags. Only -a (append) is supported.
// Uses manual parsing matching cmd/cat and cmd/wc patterns.
func parseArgs(args []string) (config, string) {
	var cfg config
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if arg == "-" || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			continue
		}

		if strings.HasPrefix(arg, "--") {
			name := arg[2:]
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '--%s'\n", progName, name)
			os.Exit(1)
		}

		// Short flags may be combined.
		for _, ch := range arg[1:] {
			switch ch {
			case 'a':
				cfg.appendMode = true
			default:
				fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, ch)
				os.Exit(1)
			}
		}
	}

	var outputFile string
	if len(files) > 0 {
		outputFile = files[0]
	}

	return cfg, outputFile
}

// registerTempFile adds a path to the global cleanup list.
func registerTempFile(path string) {
	tempMu.Lock()
	tempPaths = append(tempPaths, path)
	tempMu.Unlock()
}

// unregisterTempFile removes a path from the global cleanup list after a
// successful rename so it is not deleted again.
func unregisterTempFile(path string) {
	tempMu.Lock()
	for i, p := range tempPaths {
		if p == path {
			tempPaths = append(tempPaths[:i], tempPaths[i+1:]...)
			break
		}
	}
	tempMu.Unlock()
}

// cleanupAllTempFiles removes all registered temp files (R5.4).
func cleanupAllTempFiles() {
	tempMu.Lock()
	paths := tempPaths
	tempPaths = nil
	tempMu.Unlock()
	for _, p := range paths {
		os.Remove(p) // best-effort cleanup
	}
}

// installCleanupHandler registers signal handlers to clean up temp files
// on SIGINT, SIGTERM, and SIGHUP (R1.5).
func installCleanupHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-c
		cleanupAllTempFiles()
		os.Exit(1)
	}()
}

// memoryThreshold returns the byte count above which stdin is spilled to a
// temp file. Uses a quarter of RLIMIT_DATA when finite, otherwise 256 MB
// (R1.3).
func memoryThreshold() int64 {
	var rlim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_DATA, &rlim); err == nil && rlim.Cur > 0 && rlim.Cur < 1<<62 {
		return int64(rlim.Cur / 4)
	}
	return 256 * 1024 * 1024
}

// createTempFile creates a temp file in TMPDIR or /tmp (R1.4).
func createTempFile() (*os.File, error) {
	dir := os.Getenv("TMPDIR")
	if dir == "" {
		dir = "/tmp"
	}
	return os.CreateTemp(dir, "sponge.")
}

// soakStdin reads all of stdin into memory, spilling to a temp file if the
// buffer exceeds the memory threshold (R1.1, R1.2, R1.3). Returns either
// in-memory data (buf non-nil, tmpPath empty) or a temp file path (buf nil,
// tmpPath non-empty).
func soakStdin() ([]byte, string, error) {
	threshold := memoryThreshold()
	buf := make([]byte, 0, initialBufSize)
	chunk := make([]byte, initialBufSize)
	var tmpFile *os.File
	var tmpPath string

	for {
		n, err := os.Stdin.Read(chunk)
		if n > 0 {
			if tmpFile != nil {
				// Already spilling to temp file.
				if _, werr := tmpFile.Write(chunk[:n]); werr != nil {
					tmpFile.Close()
					return nil, tmpPath, fmt.Errorf("writing to temp file: %w", werr)
				}
			} else {
				buf = append(buf, chunk[:n]...)
				if int64(len(buf)) > threshold {
					// Spill to temp file (R1.3).
					var cerr error
					tmpFile, cerr = createTempFile()
					if cerr != nil {
						return nil, "", fmt.Errorf("creating temp file: %w", cerr)
					}
					tmpPath = tmpFile.Name()
					registerTempFile(tmpPath)
					if _, werr := tmpFile.Write(buf); werr != nil {
						tmpFile.Close()
						return nil, tmpPath, fmt.Errorf("writing to temp file: %w", werr)
					}
					buf = nil // Release memory.
				}
			}
		}
		if err == io.EOF {
			if tmpFile != nil {
				if cerr := tmpFile.Close(); cerr != nil {
					return nil, tmpPath, fmt.Errorf("closing temp file: %w", cerr)
				}
				return nil, tmpPath, nil
			}
			return buf, "", nil
		}
		if err != nil {
			if tmpFile != nil {
				tmpFile.Close()
			}
			return nil, tmpPath, err
		}
	}
}

// writePassthrough writes buffered stdin to stdout (R4.1, R4.2, R4.3).
func writePassthrough(buf []byte, tmpPath string) error {
	if tmpPath != "" {
		// Large input was spilled to temp file (R4.2).
		f, err := os.Open(tmpPath)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(os.Stdout, f)
		return err
	}
	// Small input in memory (R4.3).
	_, err := os.Stdout.Write(buf)
	return err
}

// writeToFile writes stdin content to the output file via a temp file for
// atomicity, optionally prepending original file content in append mode
// (R2, R3).
func writeToFile(cfg config, buf []byte, soakTmpPath, outputPath string) error {
	// Read existing file metadata before any writes (R2.3, R2.4).
	var origMode os.FileMode
	fileExists := false
	info, err := os.Lstat(outputPath)
	if err == nil {
		fileExists = true
		origMode = info.Mode().Perm()
	}

	// When not in append mode and data is already in a soak temp file,
	// reuse it directly to avoid an extra copy.
	if !cfg.appendMode && soakTmpPath != "" {
		return finishWrite(soakTmpPath, outputPath, origMode, fileExists)
	}

	// Create temp file for the final output content (R2.1).
	tmpFile, err := createTempFile()
	if err != nil {
		return fmt.Errorf("%s: %v", outputPath, err)
	}
	tmpOutPath := tmpFile.Name()
	registerTempFile(tmpOutPath)

	// If append mode and file exists, copy original content first (R3.1, R3.3).
	if cfg.appendMode && fileExists {
		if err := copyOriginalContent(tmpFile, outputPath); err != nil {
			tmpFile.Close()
			return err
		}
	}

	// Write stdin content to temp file.
	if err := writeStdinContent(tmpFile, buf, soakTmpPath); err != nil {
		tmpFile.Close()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("%s: %v", outputPath, err)
	}

	return finishWrite(tmpOutPath, outputPath, origMode, fileExists)
}

// finishWrite applies permissions and renames the temp file to the output
// path, falling back to a byte-for-byte copy on cross-device failure
// (R2.1, R2.2, R2.3).
func finishWrite(tmpPath, outputPath string, origMode os.FileMode, fileExists bool) error {
	mode := origMode
	if !fileExists {
		mode = defaultFileMode()
	}

	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("%s: %v", outputPath, err)
	}

	if err := os.Rename(tmpPath, outputPath); err != nil {
		// Cross-device fallback (R2.2).
		if cpErr := copyFileContent(tmpPath, outputPath, mode); cpErr != nil {
			return cpErr
		}
		os.Remove(tmpPath) // best-effort cleanup of temp after copy
	}

	unregisterTempFile(tmpPath)
	return nil
}

// defaultFileMode returns 0666 masked by the process umask (R2.3).
func defaultFileMode() os.FileMode {
	mask := syscall.Umask(0)
	syscall.Umask(mask)
	return 0666 &^ os.FileMode(mask)
}

// copyOriginalContent copies the existing output file content into dst for
// append mode (R3.3).
func copyOriginalContent(dst *os.File, srcPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("%s: %v", srcPath, err)
	}
	defer src.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("%s: %v", srcPath, err)
	}
	return nil
}

// writeStdinContent writes stdin data (from memory or soak temp file) to dst.
func writeStdinContent(dst *os.File, buf []byte, soakTmpPath string) error {
	if soakTmpPath != "" {
		src, err := os.Open(soakTmpPath)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(dst, src)
		return err
	}
	_, err := dst.Write(buf)
	return err
}

// copyFileContent copies src to dst when rename fails (cross-device, R2.2).
func copyFileContent(srcPath, dstPath string, mode os.FileMode) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("%s: %v", dstPath, err)
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("%s: %v", dstPath, err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return fmt.Errorf("%s: %v", dstPath, err)
	}

	return dst.Close()
}
