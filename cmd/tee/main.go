// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd017-tee R1.1–R1.4: core tee copy behavior.
// Reads stdin and writes to stdout and zero or more named files simultaneously.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "tee"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments, opens output files, and copies stdin to all destinations.
// Returns 0 on success, 1 on any error.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	return copyToAll(files, stdin, stdout, stderr)
}

// parseArgs separates flags from file arguments.
// Returns file list and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) ([]string, int) {
	var files []string
	flagsDone := false
	for _, arg := range args {
		if flagsDone || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "-" {
			// R1.4: "-" is an additional stdout reference
			files = append(files, arg)
			continue
		}
		code := applyFlag(arg, stdout, stderr)
		if code >= 0 {
			return nil, code
		}
	}
	return files, -1
}

// applyFlag handles a flag argument. Returns exit code >= 0 for
// terminal flags, -1 to continue.
func applyFlag(arg string, stdout, stderr io.Writer) int {
	switch arg {
	case "--help":
		printHelp(stdout)
		return 0
	case "--version":
		printVersion(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	}
}

// copyToAll opens output files and copies stdin to stdout and all files.
// R1.1: writes same bytes to stdout and all named files.
// R1.2: with no files, copies stdin to stdout only.
// R1.3: creates files that do not exist; truncates existing files.
// R1.4: "-" is treated as an additional stdout reference (not duplicated).
func copyToAll(files []string, stdin io.Reader, stdout, stderr io.Writer) int {
	writers := []io.Writer{stdout}
	exitCode := 0

	openFiles, code := openOutputFiles(files, stderr)
	if code != 0 {
		exitCode = code
	}
	defer closeFiles(openFiles)

	for _, f := range openFiles {
		writers = append(writers, f)
	}

	mw := io.MultiWriter(writers...)
	if _, err := io.Copy(mw, stdin); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		exitCode = 1
	}
	return exitCode
}

// openOutputFiles opens each named file for writing.
// R1.3: uses os.Create (O_WRONLY|O_CREATE|O_TRUNC) for default truncate mode.
// R1.4: "-" maps to stdout (skipped, already in writer list).
func openOutputFiles(files []string, stderr io.Writer) ([]*os.File, int) {
	var opened []*os.File
	exitCode := 0
	for _, name := range files {
		if name == "-" {
			// R1.4: stdout already included; skip to avoid duplicate writes
			continue
		}
		f, err := os.Create(name)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		opened = append(opened, f)
	}
	return opened, exitCode
}

// closeFiles closes all opened file handles.
func closeFiles(files []*os.File) {
	for _, f := range files {
		f.Close() // best-effort close
	}
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [FILE]...\n", progName)
	fmt.Fprintln(w, "Copy standard input to each FILE, and also to standard output.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "      --help     display this help and exit")
	fmt.Fprintln(w, "      --version  output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
