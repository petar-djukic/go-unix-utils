// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sync implements GNU sync: synchronize cached writes to persistent storage.
//
// Implements prd085-sync R1.1, R1.2, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "sync"

func main() {
	// R1.3 / R2.3: install SIGPIPE handler for graceful pipe close.
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses arguments and dispatches to the appropriate sync mode.
func run(args []string, stdout, stderr *os.File) int {
	// R1.4: handle --help and --version before other processing.
	if handleSpecialFlags(args, stdout) {
		return 0
	}

	files := stripDashDash(args)

	if len(files) == 0 {
		// R1.1: no arguments — call sync(2) to flush all filesystem caches.
		syscall.Sync()
		return 0
	}

	// R1.2: open each file and call fsync(2).
	return syncFiles(files, stderr)
}

// handleSpecialFlags checks for --help and --version, printing output and
// returning true if one was found.
func handleSpecialFlags(args []string, stdout *os.File) bool {
	for _, arg := range args {
		switch arg {
		case "--help":
			printHelp(stdout)
			return true
		case "--version":
			printVersion(stdout)
			return true
		case "--":
			return false
		}
	}
	return false
}

// stripDashDash removes the first "--" sentinel from args.
func stripDashDash(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			return append(args[:i], args[i+1:]...)
		}
	}
	return args
}

// syncFiles opens each file and calls fsync(2), reporting errors to stderr.
// R2.1: returns 0 when all operations succeed.
// R2.2: returns 1 when any operation fails.
func syncFiles(files []string, stderr *os.File) int {
	exitCode := 0
	for _, path := range files {
		if err := syncOneFile(path, stderr); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

// syncOneFile opens a single file, calls fsync, and reports any error.
func syncOneFile(path string, stderr *os.File) error {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: error opening '%s': %s\n", progName, path, unwrapErr(err)) //nolint:errcheck
		return err
	}
	defer f.Close() // best-effort close, error ignored
	if err := f.Sync(); err != nil {
		fmt.Fprintf(stderr, "%s: error syncing '%s': %s\n", progName, path, unwrapErr(err)) //nolint:errcheck
		return err
	}
	return nil
}

// unwrapErr extracts the innermost error message, stripping os.PathError wrapper.
func unwrapErr(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// printHelp outputs usage information matching GNU sync format.
func printHelp(w *os.File) {
	fmt.Fprintln(w, "Usage: sync [OPTION] [FILE]...")                          //nolint:errcheck
	fmt.Fprintln(w, "Synchronize cached writes to persistent storage.")        //nolint:errcheck
	fmt.Fprintln(w, "")                                                        //nolint:errcheck
	fmt.Fprintln(w, "If one or more files are specified, sync only them,")     //nolint:errcheck
	fmt.Fprintln(w, "or their containing file systems.")                       //nolint:errcheck
	fmt.Fprintln(w, "")                                                        //nolint:errcheck
	fmt.Fprintln(w, "      --help        display this help and exit")          //nolint:errcheck
	fmt.Fprintln(w, "      --version     output version information and exit") //nolint:errcheck
}

// printVersion outputs version information.
func printVersion(w *os.File) {
	fmt.Fprintf(w, "%s (go-unix-utils) 1.0\n", progName) //nolint:errcheck
}
