// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd017-tee R1.1–R1.5, R2.1–R2.3
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R1.5: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	// Parse flags: -a/--append, -i/--ignore-interrupts.
	appendMode, ignoreInt, fileArgs := parseArgs(os.Args[1:])

	// R2.2: When -i is given, ignore SIGINT so tee continues until EOF or
	// write error, matching GNU tee behavior.
	if ignoreInt {
		signal.Ignore(syscall.SIGINT)
	}

	// R1.1, R1.2: Build a list of writers: always include stdout.
	writers := []io.Writer{os.Stdout}
	var files []*os.File

	exitCode := 0

	// R1.3, R2.1: Choose open flags based on -a.
	openFlags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if appendMode {
		openFlags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}

	for _, arg := range fileArgs {
		// R1.4: "-" is an additional reference to stdout; data is not duplicated
		// because io.MultiWriter handles it.
		if arg == "-" {
			continue
		}
		f, err := os.OpenFile(arg, openFlags, 0o666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tee: %v\n", err)
			exitCode = 1
			continue
		}
		files = append(files, f)
		writers = append(writers, f)
	}

	// R1.1, R1.5: Read all of stdin and write to stdout and each file
	// simultaneously, preserving order.
	mw := io.MultiWriter(writers...)
	if _, err := io.Copy(mw, os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "tee: %v\n", err)
		exitCode = 1
	}

	// Close all opened files.
	for _, f := range files {
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "tee: %v\n", err)
			exitCode = 1
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// parseArgs extracts -a/--append and -i/--ignore-interrupts flags from args,
// returning the flag states and remaining file arguments. Supports combined
// short flags (e.g., -ai) and -- to end flag parsing.
func parseArgs(args []string) (appendMode, ignoreInt bool, files []string) {
	endFlags := false
	for _, arg := range args {
		if endFlags {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endFlags = true
			continue
		}
		if arg == "--append" {
			appendMode = true
			continue
		}
		if arg == "--ignore-interrupts" {
			ignoreInt = true
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			// Parse combined short flags (e.g., -ai, -ia).
			knownFlags := true
			for _, ch := range arg[1:] {
				switch ch {
				case 'a':
					appendMode = true
				case 'i':
					ignoreInt = true
				default:
					knownFlags = false
				}
			}
			if knownFlags {
				continue
			}
		}
		files = append(files, arg)
	}
	return appendMode, ignoreInt, files
}
