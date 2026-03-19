// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd017-tee R1.1–R1.5, R2.1–R2.3: tee copy behavior with
// append mode (-a) and SIGINT suppression (-i).
// Reads stdin and writes to stdout and zero or more named files simultaneously.
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "tee"

// teeOpts holds parsed command-line options.
type teeOpts struct {
	appendMode       bool // R2.1: -a / --append
	ignoreInterrupts bool // R2.2: -i / --ignore-interrupts
	files            []string
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments, opens output files, and copies stdin to all destinations.
// Returns 0 on success, 1 on any error.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}

	// R2.2: ignore SIGINT when -i is given
	if opts.ignoreInterrupts {
		installSIGINTIgnore()
	}

	return copyToAll(opts, stdin, stdout, stderr)
}

// installSIGINTIgnore causes the process to ignore SIGINT.
// R2.2: tee continues reading until EOF or write error.
func installSIGINTIgnore() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT)
	go func() {
		for range ch {
			// discard SIGINT signals
		}
	}()
}

// parseArgs separates flags from file arguments.
// Returns opts and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (teeOpts, int) {
	var opts teeOpts
	flagsDone := false

	for _, arg := range args {
		if flagsDone || len(arg) == 0 || arg[0] != '-' {
			opts.files = append(opts.files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "-" {
			// R1.4: "-" is an additional stdout reference
			opts.files = append(opts.files, arg)
			continue
		}
		code := applyFlag(arg, &opts, stdout, stderr)
		if code >= 0 {
			return opts, code
		}
	}
	return opts, -1
}

// applyFlag handles a single flag argument. Returns exit code >= 0 for
// terminal flags, -1 to continue. Supports short flag clusters (e.g. -ai).
func applyFlag(arg string, opts *teeOpts, stdout, stderr io.Writer) int {
	switch arg {
	case "--help":
		printHelp(stdout)
		return 0
	case "--version":
		printVersion(stdout)
		return 0
	case "--append":
		opts.appendMode = true
		return -1
	case "--ignore-interrupts":
		opts.ignoreInterrupts = true
		return -1
	default:
		return applyShortFlags(arg, opts, stderr)
	}
}

// applyShortFlags processes a short flag cluster like -a, -i, or -ai.
// Returns exit code >= 0 on error, -1 to continue.
func applyShortFlags(arg string, opts *teeOpts, stderr io.Writer) int {
	for _, ch := range arg[1:] {
		switch ch {
		case 'a':
			opts.appendMode = true
		case 'i':
			opts.ignoreInterrupts = true
		default:
			fmt.Fprintf(stderr, "%s: unrecognized option '-%c'\n", progName, ch)
			printTryHelp(stderr)
			return 1
		}
	}
	return -1
}

// copyToAll opens output files and copies stdin to stdout and all files.
// R1.1: writes same bytes to stdout and all named files.
// R1.2: with no files, copies stdin to stdout only.
// R1.3: creates files that do not exist; truncates or appends per mode.
// R1.4: "-" is treated as an additional stdout reference (not duplicated).
// R1.5: writes output in the order received from stdin.
func copyToAll(opts teeOpts, stdin io.Reader, stdout, stderr io.Writer) int {
	writers := []io.Writer{stdout}
	exitCode := 0

	openFiles, code := openOutputFiles(opts.files, opts.appendMode, stderr)
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
// R1.3: truncates by default; R2.1: appends when appendMode is true.
// R1.4: "-" maps to stdout (skipped, already in writer list).
func openOutputFiles(files []string, appendMode bool, stderr io.Writer) ([]*os.File, int) {
	var opened []*os.File
	exitCode := 0
	for _, name := range files {
		if name == "-" {
			// R1.4: stdout already included; skip to avoid duplicate writes
			continue
		}
		f, err := openFile(name, appendMode)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		opened = append(opened, f)
	}
	return opened, exitCode
}

// openFile opens a file for writing, using append or truncate mode.
func openFile(name string, appendMode bool) (*os.File, error) {
	if appendMode {
		// R2.1: O_APPEND preserves existing content
		return os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o666)
	}
	return os.Create(name)
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
	fmt.Fprintln(w, "  -a, --append              append to the given FILEs, do not overwrite")
	fmt.Fprintln(w, "  -i, --ignore-interrupts   ignore interrupt signals")
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
