// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/rm implements srd058-rm: remove files or directories.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type interactiveMode int

const (
	interactiveNever  interactiveMode = iota // -f / --interactive=never
	interactiveOnce                          // -I / --interactive=once
	interactiveAlways                        // -i / --interactive=always
)

type options struct {
	recursive   bool            // -r, -R, --recursive (R2.1)
	force       bool            // -f, --force (R2.2)
	dir         bool            // -d, --dir (R2.4)
	interactive interactiveMode // -i, -I, --interactive=WHEN (R3.1, R3.2, R3.4)
	verbose     bool            // -v, --verbose (R3.3)
}

var stdinScanner = bufio.NewScanner(os.Stdin)

func main() {
	sys.InstallSIGPIPEHandler()
	files, opts := parseFlags(os.Args[1:])

	if len(files) == 0 {
		if !opts.force {
			fmt.Fprintf(os.Stderr, "rm: missing operand\n")
			os.Exit(1)
		}
		return
	}

	code := removeFiles(files, opts)
	if code != 0 {
		os.Exit(code)
	}
}

func parseFlags(args []string) ([]string, options) {
	var files []string
	var opts options
	endOfFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags || arg == "" || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--recursive":
				opts.recursive = true
			case arg == "--force":
				opts.force = true
				opts.interactive = interactiveNever
			case arg == "--dir":
				opts.dir = true
			case arg == "--verbose":
				opts.verbose = true
			case arg == "--interactive=never":
				opts.force = true
				opts.interactive = interactiveNever
			case arg == "--interactive=once":
				opts.interactive = interactiveOnce
			case arg == "--interactive=always":
				opts.interactive = interactiveAlways
			case arg == "--interactive":
				opts.interactive = interactiveAlways
			}
			continue
		}
		chars := arg[1:]
		for j := 0; j < len(chars); j++ {
			switch chars[j] {
			case 'r', 'R':
				opts.recursive = true
			case 'f':
				opts.force = true
				opts.interactive = interactiveNever
			case 'd':
				opts.dir = true
			case 'i':
				opts.interactive = interactiveAlways
				opts.force = false
			case 'I':
				opts.interactive = interactiveOnce
				opts.force = false
			case 'v':
				opts.verbose = true
			}
		}
	}
	return files, opts
}

// removeFiles iterates over paths and removes each. (R1.1, R1.4, R4.1, R4.2)
func removeFiles(paths []string, opts options) int {
	fmt.Fprintf(os.Stderr, "rm: not implemented\n")
	return 1
}

// removeFile removes a single file via unlink. (R1.1, R1.4)
func removeFile(path string, opts options) error {
	return fmt.Errorf("not implemented")
}

// removeDir recursively removes a directory tree. (R2.1, R2.3)
func removeDir(path string, opts options) error {
	return fmt.Errorf("not implemented")
}

// removeEmptyDir removes an empty directory. (R2.4)
func removeEmptyDir(path string, opts options) error {
	return fmt.Errorf("not implemented")
}

// isDotOrDotDot returns true for "." and ".." path components. (R1.3)
func isDotOrDotDot(path string) bool {
	return path == "." || path == ".."
}

// promptRemoval asks the user for confirmation before removal. (R3.1, R3.2)
func promptRemoval(path string, isDir bool) bool {
	return false
}

// shouldPromptOnce returns true when -I threshold is met. (R3.2)
func shouldPromptOnce(paths []string, opts options) bool {
	return false
}
