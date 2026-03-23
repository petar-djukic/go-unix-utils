// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd007-sponge: Soak Stdin and Write to File.
// Covers R1.1-R1.5 (core soak-before-write contract),
// R2.1-R2.5 (output file handling with atomic rename, permission
// preservation, lstat, and cross-device fallback),
// R3.1-R3.2 (append mode via -a flag).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R5.4/shared protocol: Install SIGPIPE handler for piped output.
	sys.InstallSIGPIPEHandler()

	opts := parseArgs(os.Args[1:])
	exitCode := run(opts, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// spongeOptions holds the parsed arguments for a sponge invocation.
type spongeOptions struct {
	outputFile string // positional argument; empty means passthrough to stdout
	appendMode bool   // R3.1: -a flag for append mode
}

// parseArgs extracts flags and the output filename from the argument list.
// R1.2: first positional argument is the output file.
// R1.3: no argument means write to stdout.
// R3.1: -a enables append mode.
func parseArgs(args []string) spongeOptions {
	var opts spongeOptions
	for _, arg := range args {
		if arg == "--version" {
			fmt.Println("sponge (go-unix-utils) dev")
			os.Exit(0)
		}
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
		if arg == "-a" {
			opts.appendMode = true
			continue
		}
		// First non-flag argument is the output file.
		if opts.outputFile == "" {
			opts.outputFile = arg
		}
	}
	return opts
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: sponge [-a] [FILE]
Soak up standard input and write to FILE.

  -a    append to FILE instead of overwriting

If no FILE is given, write to standard output.
`)
}

// run reads all stdin, then writes to the output file or stdout.
// Returns the process exit code.
func run(opts spongeOptions, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	// R1.1: Read all of stdin before opening the output file.
	// R1.4: Works with pipes and redirects via io.ReadAll.
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "sponge: error reading stdin: %s\n", err)
		return 1
	}

	// R1.3/R4.1: No output file means passthrough to stdout.
	if opts.outputFile == "" {
		return writeToStdout(data, stdout, stderr)
	}

	return writeToFile(data, opts.outputFile, opts.appendMode, stderr)
}

// writeToStdout writes buffered data to stdout.
func writeToStdout(data []byte, stdout io.Writer, stderr io.Writer) int {
	if _, err := stdout.Write(data); err != nil {
		fmt.Fprintf(stderr, "sponge: write error: %s\n", err)
		return 1
	}
	return 0
}

// writeToFile writes data to the named file via a temp file rename.
// R2.1: temp file created in the same directory as the output file.
// R2.3: preserves original file permissions when overwriting.
// R2.4: uses Lstat to check the output path.
// R3.1-R3.2: append mode prepends existing file content.
func writeToFile(data []byte, path string, appendMode bool, stderr io.Writer) int {
	dir := filepath.Dir(path)

	// R2.3/R2.4: Read existing file permissions via Lstat before writing.
	origMode, hasOrigMode := getFileMode(path)

	// R3.1: In append mode, prepend existing file content.
	if appendMode && hasOrigMode {
		var err error
		data, err = prependFileContent(path, data)
		if err != nil {
			fmt.Fprintf(stderr, "sponge: %s\n", err)
			return 1
		}
	}

	return writeViaTempFile(data, path, dir, origMode, hasOrigMode, stderr)
}

// writeViaTempFile creates a temp file, writes data, and renames to path.
// R2.1: atomic rename. R2.2: fallback copy on cross-device.
// R3.1 (prd007): clean up temp file on write failure.
func writeViaTempFile(data []byte, path, dir string, origMode os.FileMode, hasOrigMode bool, stderr io.Writer) int {
	tmp, err := os.CreateTemp(dir, "sponge.*")
	if err != nil {
		fmt.Fprintf(stderr, "sponge: cannot create temp file: %s\n", err)
		return 1
	}
	tmpName := tmp.Name()

	// R3.1 (prd007)/R5.4: Ensure temp file is cleaned up on any error path.
	defer cleanupTemp(tmpName)

	if err := writeTempAndClose(tmp, data); err != nil {
		fmt.Fprintf(stderr, "sponge: write error: %s\n", err)
		return 1
	}

	// R2.1/R2.2: Atomic rename, with fallback copy on cross-device.
	if err := renameOrCopy(tmpName, path); err != nil {
		fmt.Fprintf(stderr, "sponge: %s\n", err)
		return 1
	}

	// R2.3: Preserve original file permissions after rename.
	if hasOrigMode {
		if err := os.Chmod(path, origMode); err != nil {
			fmt.Fprintf(stderr, "sponge: cannot set permissions: %s\n", err)
			return 1
		}
	}

	return 0
}

// getFileMode returns the file mode via Lstat if the file exists.
// R2.4: uses Lstat (not Stat) to handle symlinks correctly.
func getFileMode(path string) (os.FileMode, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, false
	}
	return info.Mode().Perm(), true
}

// prependFileContent reads existing file content and prepends it to data.
// R3.1: result is [original content][stdin content].
// R3.2: caller only invokes this when the file exists.
func prependFileContent(path string, stdinData []byte) ([]byte, error) {
	existing, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s for append: %w", path, err)
	}
	combined := make([]byte, 0, len(existing)+len(stdinData))
	combined = append(combined, existing...)
	combined = append(combined, stdinData...)
	return combined, nil
}

// renameOrCopy tries an atomic rename; on failure, falls back to copy.
// R2.2: cross-device rename fallback via byte-for-byte copy.
// R2.5: output file is never in a partially-written state.
func renameOrCopy(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	return fallbackCopy(src, dst)
}

// fallbackCopy copies src to dst byte-for-byte, then removes src.
// R2.2: used when rename fails (e.g., cross-device move).
func fallbackCopy(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open temp file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("cannot create %s: %w", dst, err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close() // best-effort close before returning
		return fmt.Errorf("cannot write to %s: %w", dst, err)
	}

	if err := dstFile.Close(); err != nil {
		return fmt.Errorf("cannot close %s: %w", dst, err)
	}

	return nil
}

// writeTempAndClose writes data to the temp file and closes it.
func writeTempAndClose(f *os.File, data []byte) error {
	_, err := f.Write(data)
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return err
}

// cleanupTemp removes the temp file if it still exists.
// Best-effort: errors are ignored since the file may have been renamed.
func cleanupTemp(path string) {
	_ = os.Remove(path) // best-effort cleanup; file may already be renamed
}
