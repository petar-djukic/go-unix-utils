// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd009-du R1.1–R1.4: core recursive disk usage traversal.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "du"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run processes path arguments and returns the exit code.
// R1.1: defaults to "." when no arguments are given.
func run(args []string, stdout, stderr io.Writer) int {
	paths := args
	if len(paths) == 0 {
		paths = []string{"."}
	}
	exitCode := 0
	for _, p := range paths {
		if err := duPath(p, stdout, stderr); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

// duPath handles a single path argument. R1.4: uses Lstat to avoid
// following symbolic links.
func duPath(path string, stdout, stderr io.Writer) error {
	fi, err := sys.Lstat(path)
	if err != nil {
		reportErr(stderr, path, err)
		return err
	}
	if !fi.Mode.IsDir() {
		printEntry(stdout, fi.Blocks, path)
		return nil
	}
	_, err = walkDir(path, fi.Blocks, stdout, stderr)
	return err
}

// walkDir recursively accumulates disk usage for a directory.
// Returns total in 512-byte blocks. R1.1: prints each subdirectory
// after its children (depth-first order).
func walkDir(dirPath string, dirBlocks int64, stdout, stderr io.Writer) (int64, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		reportErr(stderr, dirPath, err)
		printEntry(stdout, dirBlocks, dirPath)
		return dirBlocks, err
	}
	childBlocks, walkErr := processEntries(dirPath, entries, stdout, stderr)
	total := dirBlocks + childBlocks
	printEntry(stdout, total, dirPath)
	return total, walkErr
}

// processEntries iterates directory entries and accumulates block counts.
// Returns the total 512-byte blocks from all children.
func processEntries(dirPath string, entries []os.DirEntry, stdout, stderr io.Writer) (int64, error) {
	var total int64
	var hadErr error
	for _, entry := range entries {
		entryPath := joinPath(dirPath, entry.Name())
		fi, err := sys.Lstat(entryPath) // R1.4: do not follow symlinks
		if err != nil {
			reportErr(stderr, entryPath, err)
			hadErr = err
			continue
		}
		if fi.Mode.IsDir() {
			sub, err := walkDir(entryPath, fi.Blocks, stdout, stderr)
			if err != nil {
				hadErr = err
			}
			total += sub
		} else {
			total += fi.Blocks
		}
	}
	return total, hadErr
}

// printEntry outputs one du line. R1.2: converts 512-byte blocks to 1K
// blocks. R1.3: format is "SIZE\tPATH\n".
func printEntry(w io.Writer, blocks512 int64, path string) {
	fmt.Fprintf(w, "%d\t%s\n", blocks512/2, path)
}

// joinPath joins a directory and entry name without cleaning the path,
// preserving prefixes like "./" that filepath.Join would remove.
func joinPath(dir, name string) string {
	return dir + string(os.PathSeparator) + name
}

// reportErr writes a diagnostic to stderr for a path error.
func reportErr(w io.Writer, path string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(w, "%s: cannot access '%s': %s\n", progName, path, pathErr.Err)
		return
	}
	fmt.Fprintf(w, "%s: %s: %s\n", progName, path, err)
}
