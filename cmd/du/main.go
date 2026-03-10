// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd009-du R1.1–R1.5 (directory traversal, size accumulation,
// output format, symlink handling, multiple arguments).
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R5.1: exit 0 on SIGPIPE when stdout is closed by a downstream consumer.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	// R1.1: default to current directory when no arguments given.
	if len(args) == 0 {
		args = []string{"."}
	}

	exitCode := 0
	// R1.5: process arguments in command-line order.
	for _, arg := range args {
		if duArg(arg) {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// duArg processes a single command-line argument. Returns true if any error
// occurred. R1.2: accepts file or directory paths.
func duArg(path string) bool {
	fi, err := sys.Lstat(path)
	if err != nil {
		printErr("cannot access '%s': %s", path, errReason(err))
		return true
	}

	if !fi.Mode.IsDir() {
		// R1.2: for a file, print only its block count.
		fmt.Printf("%d\t%s\n", fi.Blocks/2, path)
		return false
	}

	_, hadError := walkDir(path, fi.Blocks)
	return hadError
}

// walkDir recursively traverses a directory and prints disk usage for each
// subdirectory. dirBlocks is the directory entry's own st_blocks value.
// Returns total 512-byte blocks accumulated and whether any error occurred.
// R1.1, R1.3.
func walkDir(dirPath string, dirBlocks int64) (int64, bool) {
	total := dirBlocks
	hadError := false

	f, err := os.Open(dirPath)
	if err != nil {
		printErr("cannot read directory '%s': %s", dirPath, errReason(err))
		// R1.3: print directory's own blocks even on read error.
		fmt.Printf("%d\t%s\n", total/2, dirPath)
		return total, true
	}
	// Read all entries in filesystem order to match GNU du traversal order.
	names, err := f.Readdirnames(-1)
	f.Close() // best-effort close after reading all entries
	if err != nil {
		printErr("cannot read directory '%s': %s", dirPath, errReason(err))
		hadError = true
	}

	for _, name := range names {
		childPath := dirPath + "/" + name
		childFI, err := sys.Lstat(childPath)
		if err != nil {
			printErr("cannot access '%s': %s", childPath, errReason(err))
			hadError = true
			continue
		}

		if childFI.Mode.IsDir() {
			subtotal, subErr := walkDir(childPath, childFI.Blocks)
			total += subtotal
			if subErr {
				hadError = true
			}
		} else {
			// R1.3: accumulate blocks from st_blocks via Lstat.
			total += childFI.Blocks
		}
	}

	// R1.3: size in 1K blocks. R1.4: format is SIZE\tPATH\n.
	fmt.Printf("%d\t%s\n", total/2, dirPath)
	return total, hadError
}

// printErr writes an error diagnostic to stderr. R4.2.
func printErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "du: "+format+"\n", args...)
}

// errReason extracts the human-readable reason from an OS error,
// capitalizing the first letter to match GNU coreutils strerror() output.
func errReason(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		reason := pathErr.Err.Error()
		if len(reason) > 0 && reason[0] >= 'a' && reason[0] <= 'z' {
			reason = string(reason[0]-32) + reason[1:]
		}
		return reason
	}
	return err.Error()
}
