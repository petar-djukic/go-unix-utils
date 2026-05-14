// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/du implements recursive directory disk usage reporting.
// Implements srd009-du R1.1-R1.4.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type inodeKey struct {
	Dev uint64
	Ino uint64
}

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"."}
	}

	seen := make(map[inodeKey]bool)
	exitCode := 0

	for _, arg := range args {
		if err := processArg(arg, seen); err != nil {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

func processArg(path string, seen map[inodeKey]bool) error {
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %v\n", err)
		return err
	}

	if !fi.Mode.IsDir() {
		size := fileSize(fi, seen)
		printEntry(size, path)
		return nil
	}

	_, err = walkDir(path, fi, seen)
	return err
}

func walkDir(path string, fi *sys.FileInfo, seen map[inodeKey]bool) (int64, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %v\n", err)
		size := blockSize(fi)
		printEntry(size, path)
		return size, err
	}

	total, firstErr := accumChildren(path, entries, seen)
	total += blockSize(fi)
	printEntry(total, path)
	return total, firstErr
}

func accumChildren(dir string, entries []os.DirEntry, seen map[inodeKey]bool) (int64, error) {
	var total int64
	var firstErr error

	for _, entry := range entries {
		childPath := filepath.Join(dir, entry.Name())
		size, err := childSize(childPath, seen)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		total += size
	}

	return total, firstErr
}

func childSize(path string, seen map[inodeKey]bool) (int64, error) {
	fi, err := sys.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "du: %v\n", err)
		return 0, err
	}

	if fi.Mode.IsDir() {
		return walkDir(path, fi, seen)
	}

	return fileSize(fi, seen), nil
}

func fileSize(fi *sys.FileInfo, seen map[inodeKey]bool) int64 {
	key := inodeKey{Dev: fi.Dev, Ino: fi.Ino}
	if seen[key] {
		return 0
	}
	seen[key] = true
	return blockSize(fi)
}

func blockSize(fi *sys.FileInfo) int64 {
	return fi.Blocks / 2
}

func printEntry(size int64, path string) {
	fmt.Fprintf(os.Stdout, "%d\t%s\n", size, path)
}
