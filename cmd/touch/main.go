// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd062-touch R1.1–R1.4: default file creation,
// timestamp update, multiple file arguments, and -c/--no-create.
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "touch"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stderr, time.Now()))
}

// touchOpts holds parsed command-line options for the touch utility.
type touchOpts struct {
	noCreate bool
	files    []string
}

// run parses arguments and processes each file.
// R1.1: update timestamps to current time.
// R1.2: create file if it does not exist.
// R1.3: -c suppresses creation.
// R1.4: multiple file arguments processed in order.
func run(args []string, stderr io.Writer, now time.Time) int {
	opts, code := parseArgs(args, stderr)
	if code >= 0 {
		return code
	}
	exitCode := 0
	for _, file := range opts.files {
		if err := touchFile(file, now, opts.noCreate, stderr); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}

// touchFile creates or updates timestamps for a single file.
// R1.1: updates access and modification times to now.
// R1.2: creates new empty file if it does not exist.
// R1.3: skips creation when noCreate is true.
func touchFile(path string, now time.Time, noCreate bool, stderr io.Writer) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return handleMissing(path, noCreate, now, stderr)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot touch '%s': %s\n", progName, path, stripPathError(err))
		return err
	}
	return updateTimestamps(path, now, stderr)
}

// handleMissing creates the file or skips it based on noCreate.
// R1.2: create empty file. R1.3: suppress with -c.
func handleMissing(path string, noCreate bool, now time.Time, stderr io.Writer) error {
	if noCreate {
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: cannot touch '%s': %s\n", progName, path, stripPathError(err))
		return err
	}
	f.Close() // best-effort close, file created successfully
	return updateTimestamps(path, now, stderr)
}

// updateTimestamps sets both access and modification times.
// R1.1: both times updated to the specified time.
func updateTimestamps(path string, t time.Time, stderr io.Writer) error {
	if err := os.Chtimes(path, t, t); err != nil {
		fmt.Fprintf(stderr, "%s: cannot touch '%s': %s\n", progName, path, stripPathError(err))
		return err
	}
	return nil
}

// parseArgs extracts options from args.
// Returns (opts, exitCode); exitCode -1 means continue.
func parseArgs(args []string, stderr io.Writer) (touchOpts, int) {
	var opts touchOpts
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			opts.files = append(opts.files, args[i+1:]...)
			return finishParse(opts, stderr)
		case arg == "-c" || arg == "--no-create":
			opts.noCreate = true
		case arg == "--help":
			printHelp(os.Stdout)
			return touchOpts{}, 0
		case arg == "--version":
			printVersion(os.Stdout)
			return touchOpts{}, 0
		case len(arg) > 0 && arg[0] == '-' && len(arg) > 1:
			fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
			printTryHelp(stderr)
			return touchOpts{}, 1
		default:
			opts.files = append(opts.files, arg)
		}
	}
	return finishParse(opts, stderr)
}

// finishParse validates that at least one file was provided.
func finishParse(opts touchOpts, stderr io.Writer) (touchOpts, int) {
	if len(opts.files) == 0 {
		fmt.Fprintf(stderr, "%s: missing file operand\n", progName)
		printTryHelp(stderr)
		return touchOpts{}, 1
	}
	return opts, -1
}

// stripPathError extracts the underlying message from a *os.PathError.
func stripPathError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... FILE...\n", progName)
	fmt.Fprintln(w, "Update the access and modification times of each FILE to the current time.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "A FILE argument that does not exist is created empty.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -c, --no-create  do not create any files")
	fmt.Fprintln(w, "      --help       display this help and exit")
	fmt.Fprintln(w, "      --version    output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}
