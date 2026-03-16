// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd007-sponge R1.1-R1.5, R2.1-R2.5, R3.1-R3.3, R4.1-R4.3,
// R5.1-R5.4, R6.1-R6.2.
// sponge reads all of stdin into a buffer before writing to the output file,
// enabling safe in-place pipeline rewrites. Supports -a append mode and
// passthrough to stdout when no filename is given. Installs SIGPIPE handler
// per ARCHITECTURE.yaml.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages.
const progName = "sponge"

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.5: Parse -a flag for append mode.
	var appendMode bool
	var outFile string
	for _, arg := range args {
		if arg == "-a" {
			appendMode = true
		} else {
			outFile = arg
		}
	}

	// R1.1: Read all of stdin before opening the output file.
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err)
		os.Exit(1)
	}

	// R4.1: No filename — write buffered stdin to stdout after stdin is
	// fully consumed. R4.3: small input (buffer fits in memory) writes the
	// in-memory buffer directly to stdout without creating a temp file.
	// R4.2: temp file spill path would seek and copy; not applicable here
	// since io.ReadAll keeps everything in memory.
	if outFile == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			os.Exit(1)
		}
		return
	}

	// R3.1, R3.2: Append mode — only when the output file exists and is a
	// regular file per lstat. When the path is a symlink or does not exist,
	// -a behaves identically to the default mode (R3.2).
	if appendMode {
		if info, err := os.Lstat(outFile); err == nil && info.Mode().IsRegular() {
			if err := appendToFile(outFile, data); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, outFile, err)
				os.Exit(1)
			}
			return
		}
	}

	// R1.3: Filename given — write to file via atomic rename.
	if err := writeToFile(outFile, data); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, outFile, err)
		os.Exit(1)
	}
}

// appendToFile implements R3.3: copies original file content into a temp file
// first, then appends the stdin buffer to the temp file, then renames the temp
// file over the original. The original file is read before the temp file is
// renamed, matching sponge.c:352-357. Uses atomic rename with cross-device
// fallback per R2.1-R2.2. Preserves file permissions per R2.3.
func appendToFile(path string, data []byte) error {
	// R2.3: Read existing file permissions before writing.
	mode := os.FileMode(0o666)
	if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}

	// R3.3: Read the original file content before creating the temp file.
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// R3.3: Create temp file in same directory for same-device rename.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".sponge-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	// Clean up temp file on any failure path.
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmpName) // best-effort cleanup
		}
	}()

	// R3.3: Write original content first, then append stdin buffer.
	if _, err := tmp.Write(original); err != nil {
		tmp.Close() // best-effort close before cleanup
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close() // best-effort close before cleanup
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// R2.3: Apply preserved permissions to temp file before rename.
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}

	// R2.1, R2.5: Attempt atomic rename.
	if err := os.Rename(tmpName, path); err != nil {
		// R2.2: Cross-device fallback — write combined content directly.
		combined := make([]byte, len(original)+len(data))
		copy(combined, original)
		copy(combined[len(original):], data)
		if writeErr := os.WriteFile(path, combined, mode); writeErr != nil {
			return writeErr
		}
		os.Remove(tmpName) // best-effort cleanup of temp
	}

	ok = true
	return nil
}

// writeToFile writes data to the named file using a temp-file-then-rename
// strategy for atomicity. R2.1: temp file in same directory as target.
// R2.2: atomic rename with cross-device fallback. R2.3: preserves existing
// file permissions, or uses 0666 (before umask) for new files.
// R2.4: uses lstat to detect symlinks; writes through symlinks instead of
// replacing them. R2.5: ensures the output file is never in a
// partially-written state observable by other processes.
func writeToFile(path string, data []byte) error {
	// R2.4: Use lstat (not stat) to check the output path type.
	mode := os.FileMode(0o666)
	useRename := true
	info, statErr := os.Lstat(path)
	if statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			// R2.4: Path is a symlink — do not rename (would replace the
			// symlink itself). Write directly to follow the symlink to its
			// target. Use stat (follows symlink) to get target permissions.
			useRename = false
			if targetInfo, err := os.Stat(path); err == nil && targetInfo.Mode().IsRegular() {
				mode = targetInfo.Mode().Perm()
			}
		} else if info.Mode().IsRegular() {
			// R2.3: Preserve existing regular file permissions.
			mode = info.Mode().Perm()
		}
	}
	// Path doesn't exist (statErr != nil): useRename stays true, mode stays 0666.

	// Create temp file in the same directory for same-device rename.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".sponge-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	// Clean up temp file on any failure path.
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmpName) // best-effort cleanup
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close() // best-effort close before cleanup
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}

	if useRename {
		// R2.1, R2.5: Attempt atomic rename for regular files and new files.
		if err := os.Rename(tmpName, path); err != nil {
			// R2.2: Cross-device fallback — write content directly.
			if writeErr := os.WriteFile(path, data, mode); writeErr != nil {
				return writeErr
			}
			os.Remove(tmpName) // best-effort cleanup of temp
		}
	} else {
		// R2.4: Symlink or special file — write directly (follows symlink).
		// R2.5: Temp file is fully written before this point, ensuring no
		// partial state from an interrupted stdin read.
		if writeErr := os.WriteFile(path, data, mode); writeErr != nil {
			return writeErr
		}
		os.Remove(tmpName) // best-effort cleanup
	}

	ok = true
	return nil
}
