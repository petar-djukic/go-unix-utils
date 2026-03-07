// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU tee: read stdin and write to stdout and files.
// Implements prd017-tee R1-R4.
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
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R4: Handle --help and --version before flag parsing.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Println("Usage: tee [OPTION]... [FILE]...")
			fmt.Println("Copy standard input to each FILE, and also to standard output.")
			fmt.Println()
			fmt.Println("  -a, --append              append to the given FILEs, do not overwrite")
			fmt.Println("  -i, --ignore-interrupts   ignore interrupt signals")
			fmt.Println("      --help        display this help and exit")
			fmt.Println("      --version     output version information and exit")
			os.Exit(0)
		case "--version":
			fmt.Println("tee (go-unix-utils) dev")
			os.Exit(0)
		}
	}

	// Parse flags manually.
	appendMode := false
	ignoreInterrupts := false
	var files []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "-a" || arg == "--append" {
			appendMode = true
			i++
			continue
		}
		if arg == "-i" || arg == "--ignore-interrupts" {
			ignoreInterrupts = true
			i++
			continue
		}
		// Combined short flags like -ai, -ia.
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			allFlags := true
			for _, ch := range arg[1:] {
				if ch != 'a' && ch != 'i' {
					allFlags = false
					break
				}
			}
			if allFlags {
				for _, ch := range arg[1:] {
					switch ch {
					case 'a':
						appendMode = true
					case 'i':
						ignoreInterrupts = true
					}
				}
				i++
				continue
			}
		}
		// Not a flag; stop parsing.
		break
	}

	files = args[i:]

	// R2.2: Ignore SIGINT if -i is set.
	if ignoreInterrupts {
		signal.Ignore(syscall.SIGINT)
	}

	exitCode := 0

	// Open output files.
	writers := []io.Writer{os.Stdout}
	var closers []io.Closer

	for _, path := range files {
		if path == "-" {
			// R1.4: "-" is an additional reference to stdout (no duplicate write).
			continue
		}
		flag := os.O_WRONLY | os.O_CREATE
		if appendMode {
			flag |= os.O_APPEND
		} else {
			flag |= os.O_TRUNC
		}
		f, err := os.OpenFile(path, flag, 0o666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tee: %s: %s\n", path, err.Error())
			exitCode = 1
			continue
		}
		writers = append(writers, f)
		closers = append(closers, f)
	}

	// R1.1: Fan out stdin to all writers.
	mw := io.MultiWriter(writers...)
	if _, err := io.Copy(mw, os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "tee: %s\n", err.Error())
		exitCode = 1
	}

	// Close output files.
	for _, c := range closers {
		if err := c.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "tee: %s\n", err.Error())
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}
